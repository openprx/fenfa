package handlers

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"math"
	"time"

	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/avast/apkparser"
	"github.com/gin-gonic/gin"
	"github.com/iineva/bom/pkg/asset"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"howett.net/plist"
)

// ParseAppInfo extracts metadata from APK/IPA files
func ParseAppInfo() gin.HandlerFunc {
	return func(c *gin.Context) {
		file, err := c.FormFile("app_file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "missing app_file"}})
			return
		}

		ext := strings.ToLower(filepath.Ext(file.Filename))

		var result gin.H
		var parseErr error

		switch ext {
		case ".apk":
			result, parseErr = parseAPK(file)
		case ".ipa":
			result, parseErr = parseIPA(file)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": gin.H{"code": "BAD_REQUEST", "message": "unsupported file type, must be .apk or .ipa"}})
			return
		}

		if parseErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": gin.H{"code": "PARSE_ERROR", "message": parseErr.Error()}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true, "data": result})
	}
}

// ------------------------------
// Android (APK)
// ------------------------------

func parseAPK(file *multipart.FileHeader) (gin.H, error) {
	// Save uploaded file to a temp path
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	tmp, err := os.CreateTemp("", "upload-*.apk")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return nil, err
	}
	tmp.Close()

	// Use apkparser to produce resolved AndroidManifest.xml
	var manifestBuf bytes.Buffer
	xmlEnc := xml.NewEncoder(&manifestBuf)
	xmlEnc.Indent("", "  ")
	zipErr, _, manErr := apkparser.ParseApk(path, xmlEnc)
	if zipErr != nil {
		return nil, fmt.Errorf("failed to open APK: %w", zipErr)
	}
	if manErr != nil {
		return nil, fmt.Errorf("failed to parse AndroidManifest.xml: %w", manErr)
	}
	_ = xmlEnc.Flush()

	// Parse produced XML to extract fields
	pkgName, verName, verCode := "", "", 0
	minSDK, targetSDK := "", ""
	appLabel, iconRef := "", ""

	dec := xml.NewDecoder(bytes.NewReader(manifestBuf.Bytes()))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			if name == "manifest" {
				for _, a := range t.Attr {
					if a.Name.Local == "package" {
						pkgName = a.Value
					}
					if a.Name.Local == "versionName" {
						verName = a.Value
					}
					if a.Name.Local == "versionCode" {
						if n, err := strconv.Atoi(a.Value); err == nil {
							verCode = n
						}
					}
				}
			} else if name == "uses-sdk" {
				for _, a := range t.Attr {
					if a.Name.Local == "minSdkVersion" {
						minSDK = a.Value
					}
					if a.Name.Local == "targetSdkVersion" {
						targetSDK = a.Value
					}
				}
			} else if name == "application" {
				for _, a := range t.Attr {
					if a.Name.Local == "label" && appLabel == "" {
						appLabel = a.Value
					}
					if a.Name.Local == "icon" && iconRef == "" {
						iconRef = a.Value
					}
					// Android adaptive icons may only set roundIcon
					if a.Name.Local == "roundIcon" && iconRef == "" {
						iconRef = a.Value
					}
				}
			}
		}
	}

	res := gin.H{
		"platform":     "android",
		"package_name": pkgName,
		"version_name": verName,
		"version_code": verCode,
		"app_name":     appLabel,
		"file_size":    file.Size,
		"file_name":    file.Filename,
	}
	if minSDK != "" {
		res["min_sdk"] = fmt.Sprintf("Android %s", minSDK)
	}
	if targetSDK != "" {
		res["target_sdk"] = fmt.Sprintf("Android %s", targetSDK)
	}

	// Try to extract icon from APK zip
	if iconData, _ := extractAPKIconFromPathCandidates(path, iconRef); iconData != "" {
		res["icon_base64"] = iconData
	} else if iconData := extractAPKIconHeuristic(path); iconData != "" {
		res["icon_base64"] = iconData
	}

	// Fallback label if empty: derive from package name
	if res["app_name"] == "" && pkgName != "" {
		parts := strings.Split(pkgName, ".")
		res["app_name"] = strings.Title(parts[len(parts)-1])
	}

	return res, nil
}

func extractAPKIconFromPathCandidates(apkPath, iconRef string) (string, bool) {
	if iconRef == "" {
		return "", false
	}
	// Normalize icon base name
	base := iconRef
	base = strings.TrimPrefix(base, "@")
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	if idx := strings.LastIndex(base, "."); idx >= 0 {
		base = base[idx+1:]
	}

	cands := []string{}
	dens := []string{"xxxhdpi", "xxhdpi", "xhdpi", "hdpi", "mdpi"}
	for _, d := range dens {
		for _, ext := range []string{".png", ".webp", ".jpg", ".jpeg"} {
			cands = append(cands,
				fmt.Sprintf("res/mipmap-%s/%s%s", d, base, ext),
				fmt.Sprintf("res/mipmap-%s-v26/%s%s", d, base, ext),
				fmt.Sprintf("res/drawable-%s/%s%s", d, base, ext),
				fmt.Sprintf("res/drawable-%s-v26/%s%s", d, base, ext),
			)
		}
	}
	for _, ext := range []string{".png", ".webp", ".jpg", ".jpeg"} {
		cands = append(cands, "res/drawable/"+base+ext, "res/mipmap/"+base+ext)
	}

	zr, err := zip.OpenReader(apkPath)
	if err != nil {
		return "", false
	}
	defer zr.Close()

	for _, cand := range cands {
		for _, f := range zr.File {
			if f.Name == cand {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				b, _ := io.ReadAll(rc)
				rc.Close()
				if s := imageToBase64(b); s != "" {
					return s, true
				}
			}
		}
	}
	// Try adaptive icon XML (anydpi-v26) to find foreground bitmap
	if s := extractAPKIconFromAdaptiveXML(zr, base); s != "" {
		return s, true
	}
	return "", false
}

func extractAPKIconHeuristic(apkPath string) string {
	zr, err := zip.OpenReader(apkPath)
	if err != nil {
		return ""
	}
	defer zr.Close()

	type cand struct {
		name string
		size uint64
		file *zip.File
	}
	cands := []cand{}
	re := regexp.MustCompile(`^res/(mipmap|drawable)[^/]*/(ic_?launcher(?:_round|_foreground|_monochrome)?|launcher|appicon|icon)[^/]*\.(png|webp|jpg|jpeg)$`)
	for _, f := range zr.File {
		if re.MatchString(strings.ToLower(f.Name)) {
			cands = append(cands, cand{name: f.Name, size: f.UncompressedSize64, file: f})

		}
	}
	if len(cands) == 0 {
		// Fallback: pick the largest image under res/(mipmap|drawable)
		imgRe := regexp.MustCompile(`^res/(mipmap|drawable)[^/]*\/[^/]*\.(png|webp|jpg|jpeg)$`)
		for _, f := range zr.File {
			if imgRe.MatchString(strings.ToLower(f.Name)) {
				cands = append(cands, cand{name: f.Name, size: f.UncompressedSize64, file: f})
			}
		}
		// If still none, try adaptive-icon XML → vector渲染
		if len(cands) == 0 {
			// 1) anydpi-v26 adaptive icon XMLs
			aiRe := regexp.MustCompile(`^res/(mipmap|drawable)-anydpi-v26/[^/]+\.xml$`)
			for _, f := range zr.File {
				name := strings.ToLower(f.Name)
				if aiRe.MatchString(name) {
					rc, err := f.Open()
					if err != nil {
						continue
					}
					data, _ := io.ReadAll(rc)
					rc.Close()
					if bytes.Contains(data, []byte("<adaptive-icon")) {
						// parse foreground then search bitmap or vector-render
						base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
						if s := extractAPKIconFromAdaptiveXML(zr, base); s != "" {
							return s
						}
					}
				}
			}
			// 2) common launcher/foreground vectors
			vxRe := regexp.MustCompile(`^res/(mipmap|drawable)[^/]*/(ic_?launcher(?:_round|_foreground|_monochrome)?|launcher|appicon|icon)[^/]*\.xml$`)
			for _, f := range zr.File {
				if vxRe.MatchString(strings.ToLower(f.Name)) {
					rc, err := f.Open()
					if err != nil {
						continue
					}
					xmlData, _ := io.ReadAll(rc)
					rc.Close()
					if bytes.Contains(xmlData, []byte("<vector")) {
						if s := renderVectorDrawableToPNG(xmlData, 512); s != "" {
							return s
						}
					}
				}
			}
			// Nothing found -> try generic res/* root images (obfuscated AAPT2 paths)
			// Pick the best-looking candidate by image dimensions (prefer square and medium-large)
			bestScore := -1.0
			var bestFile *zip.File
			imgRootRe := regexp.MustCompile(`^res/[^/]+\.(png|jpg|jpeg|webp)$`)
			for _, f := range zr.File {
				name := strings.ToLower(f.Name)
				if !imgRootRe.MatchString(name) {
					continue
				}
				// skip nine-patch
				if strings.HasSuffix(name, ".9.png") {
					continue
				}
				// Try to open and decode to get dimensions
				rc, err := f.Open()
				if err != nil {
					continue
				}
				b, _ := io.ReadAll(rc)
				rc.Close()
				img, _, err := image.Decode(bytes.NewReader(b))
				if err != nil {
					continue
				}
				w := img.Bounds().Dx()
				h := img.Bounds().Dy()
				if w < 96 || h < 96 {
					continue
				}
				// score: closeness to square and size towards 512
				aspect := 1.0
				if w > 0 && h > 0 {
					if w > h {
						aspect = float64(h) / float64(w)
					} else {
						aspect = float64(w) / float64(h)
					}
				}
				mh := w
				if h > w {
					mh = h
				}
				sizePref := 1.0 - math.Min(1.0, math.Abs(float64(mh-512))/512.0)
				score := aspect*0.7 + sizePref*0.3
				if score > bestScore {
					bestScore = score
					bestFile = f
				}
			}
			if bestFile != nil {
				fmt.Println("[parse-app] res root icon candidate:", bestFile.Name)
				rc, err := bestFile.Open()
				if err == nil {
					b, _ := io.ReadAll(rc)
					rc.Close()
					return imageToBase64(b)
				}
			}
			// Try common asset icons as the last resort (Flutter/React Native)
			bestScore = -1.0
			bestFile = nil
			assetRe := regexp.MustCompile(`^assets/.*/(ic_?launcher[^/]*|appicon[^/]*|icon[^/]*|logo[^/]*)\.(png|jpg|jpeg|webp)$`)
			for _, f := range zr.File {
				name := strings.ToLower(f.Name)
				if !assetRe.MatchString(name) {
					continue
				}
				rc, err := f.Open()
				if err != nil {
					continue
				}
				b, _ := io.ReadAll(rc)
				rc.Close()
				img, _, err := image.Decode(bytes.NewReader(b))
				if err != nil {
					continue
				}
				w := img.Bounds().Dx()
				h := img.Bounds().Dy()
				if w < 96 || h < 96 {
					continue
				}
				aspect := 1.0
				if w > h {
					aspect = float64(h) / float64(w)
				} else {
					aspect = float64(w) / float64(h)
				}
				mh := w
				if h > w {
					mh = h
				}
				sizePref := 1.0 - math.Min(1.0, math.Abs(float64(mh-512))/512.0)
				score := aspect*0.7 + sizePref*0.3
				if score > bestScore {
					bestScore = score
					bestFile = f
				}
			}
			if bestFile != nil {
				fmt.Println("[parse-app] assets icon candidate:", bestFile.Name)
				rc, err := bestFile.Open()
				if err == nil {
					b, _ := io.ReadAll(rc)
					rc.Close()
					return imageToBase64(b)
				}
			}

			return ""
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].size > cands[j].size })
	rc, err := cands[0].file.Open()
	if err != nil {
		return ""
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	return imageToBase64(b)
}

// ------------------------------
// iOS (IPA)
// ------------------------------

func parseIPA(file *multipart.FileHeader) (gin.H, error) {
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, src); err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		return nil, fmt.Errorf("failed to read IPA: %w", err)
	}

	var plistFile *zip.File
	var appDir string
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "Payload/") && strings.HasSuffix(f.Name, ".app/Info.plist") {
			plistFile = f
			appDir = filepath.Dir(f.Name)
			break
		}
	}
	if plistFile == nil {
		return nil, fmt.Errorf("Info.plist not found")
	}

	rc, err := plistFile.Open()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if _, err := plist.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse Info.plist: %w", err)
	}

	bundleID := getPlistString(m, "CFBundleIdentifier")
	version := getPlistString(m, "CFBundleShortVersionString")
	build := getPlistString(m, "CFBundleVersion")
	appName := getPlistString(m, "CFBundleDisplayName")
	if appName == "" {
		appName = getPlistString(m, "CFBundleName")
	}
	minOS := getPlistString(m, "MinimumOSVersion")

	res := gin.H{
		"platform":  "ios",
		"bundle_id": bundleID,
		"version":   version,
		"build":     build,
		"app_name":  appName,
		"min_os":    fmt.Sprintf("iOS %s", minOS),
		"file_size": file.Size,
		"file_name": file.Filename,
	}

	// Try extracting icon from Assets.car first (supports modern IPAs)
	if icon := extractIPAIconFromAssetsCar(zr, appDir, m); icon != "" {
		res["icon_base64"] = icon
	} else if icon := extractIPAIcon(zr, appDir, m); icon != "" { // fallback to standalone PNGs
		res["icon_base64"] = icon
	}

	// Extract provisioning profile information
	if profile := extractMobileProvision(zr, appDir); profile != nil {
		res["provisioning_profile"] = profile
	}

	return res, nil
}

func getPlistString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Parse adaptive icon XML under anydpi-v26 and composite background + foreground into a square PNG
func extractAPKIconFromAdaptiveXML(zr *zip.ReadCloser, base string) string {
	// Candidate XML paths for adaptive icons
	xmlCands := []string{
		fmt.Sprintf("res/mipmap-anydpi-v26/%s.xml", base),
		fmt.Sprintf("res/drawable-anydpi-v26/%s.xml", base),
		fmt.Sprintf("res/mipmap-anydpi/%s.xml", base),
		fmt.Sprintf("res/drawable-anydpi/%s.xml", base),
	}
	var xmlData []byte
	for _, cand := range xmlCands {
		for _, f := range zr.File {
			if f.Name == cand {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				b, _ := io.ReadAll(rc)
				rc.Close()
				xmlData = b
				break
			}
		}
		if len(xmlData) > 0 {
			break
		}
	}
	if len(xmlData) == 0 {
		return ""
	}

	// <adaptive-icon>
	//   <background android:drawable="@color/... or @drawable/... or #RRGGBB"/>
	//   <foreground android:drawable="@drawable/... or vector"/>
	// </adaptive-icon>
	dec := xml.NewDecoder(bytes.NewReader(xmlData))
	fgRef := ""
	bgRef := ""
	bgColorStr := ""
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ""
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "foreground" {
				for _, a := range t.Attr {
					if a.Name.Local == "drawable" && fgRef == "" {
						fgRef = a.Value
					}
				}
			}
			if t.Name.Local == "background" {
				for _, a := range t.Attr {
					if a.Name.Local == "drawable" && bgRef == "" && bgColorStr == "" {
						v := strings.TrimSpace(a.Value)
						// immediate color value
						if strings.HasPrefix(v, "#") || strings.HasPrefix(strings.ToLower(v), "0x") {
							bgColorStr = v
						} else {
							bgRef = v
						}
					}
				}
			}
		}
	}

	// Try to load both layers and composite
	const outSize = 512
	var bgImg image.Image
	var fgImg image.Image
	// background color (if direct)
	var bgFill *image.Uniform
	if bgColorStr != "" {
		if hex, a := parseAndroidColor(bgColorStr); hex != "" {
			s := strings.TrimPrefix(hex, "#")
			if len(s) == 6 {
				r, _ := strconv.ParseUint(s[0:2], 16, 8)
				g, _ := strconv.ParseUint(s[2:4], 16, 8)
				b, _ := strconv.ParseUint(s[4:6], 16, 8)
				bgFill = &image.Uniform{C: color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a*255 + 0.5)}}
			}
		}
	}
	if bgRef != "" {
		if img := loadDrawableImage(zr, bgRef, outSize); img != nil {
			bgImg = img
		}
	}
	if fgRef != "" {
		if img := loadDrawableImage(zr, fgRef, outSize); img != nil {
			fgImg = img
		}
	}
	// If nothing to composite, fall back to previous foreground-only logic
	if fgImg == nil && bgImg == nil && bgFill == nil {
		// normalize and try existing logic
		fg := strings.TrimPrefix(fgRef, "@")
		if idx := strings.LastIndex(fg, "/"); idx >= 0 {
			fg = fg[idx+1:]
		}
		if idx := strings.LastIndex(fg, "."); idx >= 0 {
			fg = fg[:idx]
		}
		if s := findImageByBaseInZip(zr, fg); s != "" {
			return s
		}
		if alt := resolveImageBaseFromXML(zr, fg); alt != "" {
			if s := findImageByBaseInZip(zr, alt); s != "" {
				return s
			}
			if xmlData := findVectorXMLByBaseInZip(zr, alt); xmlData != nil {
				if s := renderVectorDrawableToPNG(xmlData, outSize); s != "" {
					return s
				}
			}
		}
		if xmlData := findVectorXMLByBaseInZip(zr, fg); xmlData != nil {
			if s := renderVectorDrawableToPNG(xmlData, outSize); s != "" {
				return s
			}
		}
		return ""
	}

	// Compose canvas
	canvas := image.NewRGBA(image.Rect(0, 0, outSize, outSize))
	if bgFill != nil {
		draw.Draw(canvas, canvas.Bounds(), bgFill, image.Point{}, draw.Src)
	}
	if bgImg != nil {
		scaled := scaleImageToFit(bgImg, outSize)
		draw.Draw(canvas, canvas.Bounds(), scaled, image.Point{}, draw.Over)
	}
	if fgImg != nil {
		scaled := scaleImageToFit(fgImg, outSize)
		draw.Draw(canvas, canvas.Bounds(), scaled, image.Point{}, draw.Over)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
}

// Resolve image base referenced inside an intermediate XML drawable, e.g. <bitmap android:src="@drawable/xxx">
func resolveImageBaseFromXML(zr *zip.ReadCloser, base string) string {
	// find any res/(drawable|mipmap)*/<base>.xml
	re := regexp.MustCompile("^res/(mipmap|drawable)[^/]*/" + regexp.QuoteMeta(base) + `\.xml$`)
	for _, f := range zr.File {
		if re.MatchString(strings.ToLower(f.Name)) {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			dec := xml.NewDecoder(bytes.NewReader(data))
			for {
				tok, err := dec.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					break
				}
				switch t := tok.(type) {
				case xml.StartElement:
					// Common cases: <bitmap android:src="@drawable/xxx"/> or <inset android:drawable="@drawable/xxx"/>
					for _, a := range t.Attr {
						if (a.Name.Local == "src" || a.Name.Local == "drawable") && strings.HasPrefix(a.Value, "@") {
							ref := strings.TrimPrefix(a.Value, "@")
							if idx := strings.LastIndex(ref, "/"); idx >= 0 {
								ref = ref[idx+1:]
							}
							if idx := strings.LastIndex(ref, "."); idx >= 0 {
								ref = ref[:idx]
							}
							return ref
						}
					}
				}

			}
		}
	}
	return ""
}

// Find vector drawable XML by base name
func findVectorXMLByBaseInZip(zr *zip.ReadCloser, base string) []byte {
	re := regexp.MustCompile("^res/(mipmap|drawable)[^/]*/" + regexp.QuoteMeta(base) + `\.xml$`)
	for _, f := range zr.File {
		if re.MatchString(strings.ToLower(f.Name)) {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			b, _ := io.ReadAll(rc)
			rc.Close()
			// Quick check: must be a <vector> drawable
			if bytes.Contains(b, []byte("<vector")) {
				return b
			}
		}
	}
	return nil
}

// Render Android Vector Drawable XML to PNG (size x size), return data URL
func renderVectorDrawableToPNG(xmlData []byte, size int) string {
	svg, ok := vectorDrawableToSVG(xmlData)
	if !ok {
		return ""
	}
	icon, err := oksvg.ReadIconStream(bytes.NewReader([]byte(svg)))
	if err != nil {
		return ""
	}
	w, h := float64(size), float64(size)
	icon.SetTarget(0, 0, w, h)
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	scanner := rasterx.NewScannerGV(int(w), int(h), img, img.Bounds())
	r := rasterx.NewDasher(int(w), int(h), scanner)
	icon.Draw(r, 1.0)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
}

// Render Android Vector Drawable XML to in-memory image (size x size)
func renderVectorDrawableToImage(xmlData []byte, size int) image.Image {
	svg, ok := vectorDrawableToSVG(xmlData)
	if !ok {
		return nil
	}
	icon, err := oksvg.ReadIconStream(bytes.NewReader([]byte(svg)))
	if err != nil {
		return nil
	}
	w, h := float64(size), float64(size)
	icon.SetTarget(0, 0, w, h)
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	scanner := rasterx.NewScannerGV(int(w), int(h), img, img.Bounds())
	r := rasterx.NewDasher(int(w), int(h), scanner)
	icon.Draw(r, 1.0)
	return img
}

// Nearest-neighbor scale to fit into a size x size square, centered
func scaleImageToFit(src image.Image, size int) *image.RGBA {
	if src == nil || size <= 0 {
		return image.NewRGBA(image.Rect(0, 0, size, size))
	}
	sw := src.Bounds().Dx()
	sh := src.Bounds().Dy()
	if sw == 0 || sh == 0 {
		return image.NewRGBA(image.Rect(0, 0, size, size))
	}
	// compute fit size preserving aspect ratio
	scaleW := float64(size) / float64(sw)
	scaleH := float64(size) / float64(sh)
	s := scaleW
	if scaleH < s {
		s = scaleH
	}
	nw := int(float64(sw)*s + 0.5)
	nh := int(float64(sh)*s + 0.5)
	if nw <= 0 {
		nw = 1
	}
	if nh <= 0 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	offX := (size - nw) / 2
	offY := (size - nh) / 2
	// nearest-neighbor sampling
	for y := 0; y < nh; y++ {
		sy := y * sh / nh
		for x := 0; x < nw; x++ {
			sx := x * sw / nw
			dst.Set(offX+x, offY+y, src.At(src.Bounds().Min.X+sx, src.Bounds().Min.Y+sy))
		}
	}
	return dst
}

// Load an Android drawable reference into an image (bitmap or vector), scaled to `size`
func loadDrawableImage(zr *zip.ReadCloser, ref string, size int) image.Image {
	if ref == "" {
		return nil
	}
	// Normalize reference to base name
	base := strings.TrimPrefix(strings.TrimSpace(ref), "@")
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	if idx := strings.LastIndex(base, "."); idx >= 0 {
		base = base[:idx]
	}
	return loadDrawableImageByBase(zr, base, size, 0)
}

func loadDrawableImageByBase(zr *zip.ReadCloser, base string, size int, depth int) image.Image {
	if depth > 2 || base == "" {
		return nil
	}
	// 1) Try bitmap in any qualifiers
	re := regexp.MustCompile("^res/(mipmap|drawable)[^/]*/" + regexp.QuoteMeta(base) + `\.(png|webp|jpg|jpeg)$`)
	type cand struct {
		size uint64
		file *zip.File
	}
	cands := []cand{}
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if re.MatchString(name) {
			cands = append(cands, cand{size: f.UncompressedSize64, file: f})
		}
	}
	if len(cands) > 0 {
		sort.Slice(cands, func(i, j int) bool { return cands[i].size > cands[j].size })
		rc, err := cands[0].file.Open()
		if err == nil {
			b, _ := io.ReadAll(rc)
			rc.Close()
			if img, _, err := image.Decode(bytes.NewReader(b)); err == nil {
				return scaleImageToFit(img, size)
			}
		}
	}
	// 2) Try intermediate XML wrapper (bitmap/inset) → resolve base
	if alt := resolveImageBaseFromXML(zr, base); alt != "" {
		if img := loadDrawableImageByBase(zr, alt, size, depth+1); img != nil {
			return img
		}
	}
	// 3) Try vector drawable
	if xmlData := findVectorXMLByBaseInZip(zr, base); xmlData != nil {
		if img := renderVectorDrawableToImage(xmlData, size); img != nil {
			return img
		}
	}
	return nil
}

// Convert a subset of Android Vector Drawable to a simple SVG string
func vectorDrawableToSVG(xmlData []byte) (string, bool) {
	dec := xml.NewDecoder(bytes.NewReader(xmlData))
	var vw, vh float64 = 24, 24
	type pathDef struct {
		D           string
		Fill        string
		FillAlpha   string
		Stroke      string
		StrokeW     string
		StrokeAlpha string
		FillType    string
	}
	paths := []pathDef{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "vector" {
				for _, a := range t.Attr {
					if a.Name.Local == "viewportWidth" {
						if f, e := strconv.ParseFloat(a.Value, 64); e == nil {
							vw = f
						}
					}
					if a.Name.Local == "viewportHeight" {
						if f, e := strconv.ParseFloat(a.Value, 64); e == nil {
							vh = f
						}
					}
				}
			}
			if t.Name.Local == "path" {
				p := pathDef{}
				for _, a := range t.Attr {
					sw := a.Name.Local
					sv := a.Value
					switch sw {
					case "pathData":
						p.D = sv
					case "fillColor":
						p.Fill = sv
					case "fillAlpha":
						p.FillAlpha = sv
					case "strokeColor":
						p.Stroke = sv
					case "strokeWidth":
						p.StrokeW = sv
					case "strokeAlpha":
						p.StrokeAlpha = sv
					case "fillType":
						p.FillType = sv
					}
				}
				if p.D != "" {
					paths = append(paths, p)
				}
			}
		}
	}
	if len(paths) == 0 {
		return "", false
	}
	b := &bytes.Buffer{}
	fmt.Fprintf(b, "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 %g %g\">", vw, vh)
	for _, p := range paths {
		fill, fa := parseAndroidColor(p.Fill)
		if p.FillAlpha != "" {
			if v, e := strconv.ParseFloat(p.FillAlpha, 64); e == nil {
				fa *= v
			}
		}
		stroke, sa := parseAndroidColor(p.Stroke)
		if p.StrokeAlpha != "" {
			if v, e := strconv.ParseFloat(p.StrokeAlpha, 64); e == nil {
				sa *= v
			}
		}
		fr := ""
		if strings.EqualFold(p.FillType, "evenOdd") {
			fr = " fill-rule=\"evenodd\""
		}
		fmt.Fprintf(b, "<path d=\"%s\"", p.D)
		if fill != "" {
			fmt.Fprintf(b, " fill=\"%s\"", fill)
			if fa < 1 {
				fmt.Fprintf(b, " fill-opacity=\"%0.3f\"", fa)
			}
		} else {
			fmt.Fprint(b, " fill=\"#000\"")
		}
		if stroke != "" {
			fmt.Fprintf(b, " stroke=\"%s\"", stroke)
		}
		if p.StrokeW != "" {
			fmt.Fprintf(b, " stroke-width=\"%s\"", p.StrokeW)
		}
		if sa < 1 {
			fmt.Fprintf(b, " stroke-opacity=\"%0.3f\"", sa)
		}
		fmt.Fprintf(b, "%s/>", fr)
	}
	fmt.Fprint(b, "</svg>")
	return b.String(), true
}

// Parse Android color formats (#RRGGBB, #AARRGGBB, 0xRRGGBB, 0xAARRGGBB)
func parseAndroidColor(s string) (hex string, alpha float64) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 1
	}
	s = strings.TrimPrefix(s, "#")
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		s = s[2:]
	}
	s = strings.TrimPrefix(s, "ffFF")
	s = strings.TrimSpace(s)
	if len(s) == 8 { // AARRGGBB
		a, _ := strconv.ParseUint(s[0:2], 16, 8)
		hex = "#" + s[2:8]
		alpha = float64(a) / 255.0
		return
	}
	if len(s) == 6 {
		return "#" + s, 1
	}
	return "", 1
}

// Search drawable/mipmap anywhere for <base>.(png|webp|jpg|jpeg) across any qualifiers
func findImageByBaseInZip(zr *zip.ReadCloser, base string) string {
	// match like: res/drawable-anydpi-v26/<base>.webp or res/mipmap-xxxhdpi/<base>.png
	re := regexp.MustCompile("^res/(mipmap|drawable)[^/]*/" + regexp.QuoteMeta(base) + `\.(png|webp|jpg|jpeg)$`)
	type cand struct {
		size uint64
		file *zip.File
	}
	cands := []cand{}
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if re.MatchString(name) {
			cands = append(cands, cand{size: f.UncompressedSize64, file: f})
		}
	}
	if len(cands) == 0 {
		return ""
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].size > cands[j].size })
	rc, err := cands[0].file.Open()
	if err != nil {
		return ""
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	return imageToBase64(b)
}

func extractIPAIcon(zr *zip.Reader, appDir string, m map[string]interface{}) string {
	// Try CFBundleIcons > CFBundlePrimaryIcon > CFBundleIconFiles
	if icons, ok := m["CFBundleIcons"].(map[string]interface{}); ok {
		if prim, ok := icons["CFBundlePrimaryIcon"].(map[string]interface{}); ok {
			if files, ok := prim["CFBundleIconFiles"].([]interface{}); ok {
				cands := []string{}
				for _, it := range files {
					if name, ok := it.(string); ok {
						cands = append(cands,
							name+".png", name+"@2x.png", name+"@3x.png",
						)
					}
				}
				if s := findIPAIconByNames(zr, appDir, cands); s != "" {
					return s
				}
			}
		}
	}
	// Fallback: search common names
	common := []string{"AppIcon60x60@3x.png", "AppIcon60x60@2x.png", "AppIcon76x76@2x.png"}
	if s := findIPAIconByNames(zr, appDir, common); s != "" {
		return s
	}
	// Last resort: any png containing "appicon" or "icon"
	cands := []string{}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, appDir+"/") && strings.HasSuffix(strings.ToLower(f.Name), ".png") {
			low := strings.ToLower(filepath.Base(f.Name))
			if strings.Contains(low, "appicon") || strings.HasPrefix(low, "icon") {
				cands = append(cands, f.Name)
			}
		}
	}
	sort.Slice(cands, func(i, j int) bool { return len(cands[i]) < len(cands[j]) })
	for _, name := range cands {
		if s := readZipImageBase64(zr, name); s != "" {
			return s
		}
	}
	return ""
}

func findIPAIconByNames(zr *zip.Reader, appDir string, names []string) string {
	for _, n := range names {
		for _, f := range zr.File {
			if f.Name == appDir+"/"+n {
				if s := readZipImageBase64(zr, f.Name); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func readZipImageBase64(zr *zip.Reader, name string) string {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return ""
			}
			b, _ := io.ReadAll(rc)
			rc.Close()
			return imageToBase64(b)
		}
	}
	return ""
}

// Try to extract AppIcon from Assets.car (Asset Catalog)
func extractIPAIconFromAssetsCar(zr *zip.Reader, appDir string, m map[string]interface{}) string {
	// Locate Assets.car inside the .app bundle
	var car *zip.File
	for _, f := range zr.File {
		if f.Name == appDir+"/Assets.car" {
			car = f
			break
		}
	}
	if car == nil {
		return ""
	}

	rc, err := car.Open()
	if err != nil {
		return ""
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return ""
	}

	// Build candidate asset names
	cands := map[string]struct{}{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" {
			cands[s] = struct{}{}
		}
	}

	// CFBundleIconName at root
	if v := getPlistString(m, "CFBundleIconName"); v != "" {
		add(v)
	}
	// CFBundleIcons -> CFBundlePrimaryIcon -> CFBundleIconName / CFBundleIconFiles
	if icons, ok := m["CFBundleIcons"].(map[string]interface{}); ok {
		if prim, ok := icons["CFBundlePrimaryIcon"].(map[string]interface{}); ok {
			if nameAny, ok := prim["CFBundleIconName"]; ok {
				if s, ok := nameAny.(string); ok {
					add(s)
				}
			}
			if files, ok := prim["CFBundleIconFiles"].([]interface{}); ok {
				for _, it := range files {
					if s, ok := it.(string); ok {
						// Normalize file-like names to asset set names
						s = strings.TrimSuffix(s, ".png")
						s = strings.TrimSuffix(s, "@2x")
						s = strings.TrimSuffix(s, "@3x")
						add(s)
					}
				}
			}
		}
	}
	// Common fallbacks
	add("AppIcon")
	add("App-Icon")
	add("AppIcon60x60")
	add("AppIcon76x76")
	add("AppIcon83.5x83.5")

	// Open Assets.car with a ReadSeeker
	ac, err := asset.NewWithReadSeeker(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	// Try candidates; prefer order of insertion by iterating over keys deterministically
	for name := range cands {
		if img, err := ac.Image(name); err == nil && img != nil {
			var out bytes.Buffer
			if err := png.Encode(&out, img); err == nil {
				return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
			}
		}
	}
	return ""
}

// ------------------------------
// Shared helpers
// ------------------------------

func imageToBase64(b []byte) string {
	// Try to decode and re-encode as a standard PNG
	if img, _, err := image.Decode(bytes.NewReader(b)); err == nil {
		var out bytes.Buffer
		if err := png.Encode(&out, img); err == nil {
			return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
		}
	}
	// Fallback: label with detected content-type (e.g., image/webp)
	ct := http.DetectContentType(b)
	if !strings.HasPrefix(ct, "image/") {
		ct = "image/png"
	}
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(b)
}

// ------------------------------
// iOS Provisioning Profile Parsing
// ------------------------------

// extractMobileProvision extracts and parses the embedded.mobileprovision file from an IPA
func extractMobileProvision(zr *zip.Reader, appDir string) gin.H {
	// Find embedded.mobileprovision
	provisionPath := appDir + "/embedded.mobileprovision"
	var provisionFile *zip.File
	for _, f := range zr.File {
		if f.Name == provisionPath {
			provisionFile = f
			break
		}
	}
	if provisionFile == nil {
		return nil
	}

	rc, err := provisionFile.Open()
	if err != nil {
		return nil
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil
	}

	// Extract plist from CMS signed data (find <?xml ... </plist>)
	plistData := extractPlistFromProvision(data)
	if plistData == nil {
		return nil
	}

	// Parse plist
	var m map[string]interface{}
	if _, err := plist.Unmarshal(plistData, &m); err != nil {
		return nil
	}

	result := gin.H{}

	// Basic profile info
	if v, ok := m["UUID"].(string); ok {
		result["uuid"] = v
	}
	if v, ok := m["Name"].(string); ok {
		result["name"] = v
	}
	if v, ok := m["AppIDName"].(string); ok {
		result["app_id_name"] = v
	}
	if v, ok := m["TeamName"].(string); ok {
		result["team_name"] = v
	}

	// Team ID (first element of TeamIdentifier array)
	if teams, ok := m["TeamIdentifier"].([]interface{}); ok && len(teams) > 0 {
		if teamID, ok := teams[0].(string); ok {
			result["team_id"] = teamID
		}
	}

	// App ID prefix
	if prefixes, ok := m["ApplicationIdentifierPrefix"].([]interface{}); ok && len(prefixes) > 0 {
		if prefix, ok := prefixes[0].(string); ok {
			result["app_id_prefix"] = prefix
		}
	}

	// Dates
	if v, ok := m["CreationDate"].(time.Time); ok {
		result["creation_date"] = v
	}
	if v, ok := m["ExpirationDate"].(time.Time); ok {
		result["expiration_date"] = v
	}

	// Platform
	if platforms, ok := m["Platform"].([]interface{}); ok && len(platforms) > 0 {
		platStrs := make([]string, 0, len(platforms))
		for _, p := range platforms {
			if ps, ok := p.(string); ok {
				platStrs = append(platStrs, ps)
			}
		}
		result["platform"] = strings.Join(platStrs, ", ")
	}

	// ProvisionsAllDevices (enterprise profiles)
	if v, ok := m["ProvisionsAllDevices"].(bool); ok {
		result["provisions_all_devices"] = v
	} else {
		result["provisions_all_devices"] = false
	}

	// Entitlements
	if ent, ok := m["Entitlements"].(map[string]interface{}); ok {
		result["entitlements"] = ent
		// Extract bundle ID from application-identifier
		if appID, ok := ent["application-identifier"].(string); ok {
			// Format: TEAMID.com.example.app
			if idx := strings.Index(appID, "."); idx > 0 {
				result["bundle_id"] = appID[idx+1:]
			}
		}
	}

	// Provisioned devices (UDID list)
	if devices, ok := m["ProvisionedDevices"].([]interface{}); ok {
		deviceList := make([]string, 0, len(devices))
		for _, d := range devices {
			if udid, ok := d.(string); ok {
				deviceList = append(deviceList, udid)
			}
		}
		result["provisioned_devices"] = deviceList
		result["device_count"] = len(deviceList)
	}

	// Determine profile type
	result["profile_type"] = determineProfileType(m)

	// Parse certificates
	if certs, ok := m["DeveloperCertificates"].([]interface{}); ok {
		certInfos := parseDeveloperCertificates(certs)
		if len(certInfos) > 0 {
			result["certificates"] = certInfos
		}
	}

	return result
}

// extractPlistFromProvision extracts the plist XML from a CMS-signed mobileprovision file
func extractPlistFromProvision(data []byte) []byte {
	// Find <?xml and </plist> markers
	startMarker := []byte("<?xml")
	endMarker := []byte("</plist>")

	startIdx := bytes.Index(data, startMarker)
	if startIdx < 0 {
		return nil
	}

	endIdx := bytes.Index(data[startIdx:], endMarker)
	if endIdx < 0 {
		return nil
	}

	return data[startIdx : startIdx+endIdx+len(endMarker)]
}

// determineProfileType determines the type of provisioning profile
func determineProfileType(m map[string]interface{}) string {
	// Check for enterprise (In-House) distribution
	if v, ok := m["ProvisionsAllDevices"].(bool); ok && v {
		return "enterprise"
	}

	// Check entitlements for get-task-allow (development vs distribution)
	if ent, ok := m["Entitlements"].(map[string]interface{}); ok {
		if getTaskAllow, ok := ent["get-task-allow"].(bool); ok {
			if getTaskAllow {
				return "development"
			}
		}
		// Check for beta-reports-active (TestFlight/App Store)
		if _, ok := ent["beta-reports-active"]; ok {
			return "app-store"
		}
	}

	// Has device list but not enterprise = ad-hoc
	if devices, ok := m["ProvisionedDevices"].([]interface{}); ok && len(devices) > 0 {
		return "ad-hoc"
	}

	// Default to distribution if no devices and not development
	return "distribution"
}

// CertificateInfo holds parsed certificate information
type CertificateInfo struct {
	Name         string    `json:"name"`
	SerialNumber string    `json:"serial_number"`
	SHA1         string    `json:"sha1"`
	CreationDate time.Time `json:"creation_date"`
	ExpiryDate   time.Time `json:"expiry_date"`
}

// parseDeveloperCertificates parses the DER-encoded certificates from the profile
func parseDeveloperCertificates(certs []interface{}) []CertificateInfo {
	var result []CertificateInfo

	for _, c := range certs {
		certData, ok := c.([]byte)
		if !ok {
			continue
		}

		cert, err := x509.ParseCertificate(certData)
		if err != nil {
			continue
		}

		info := CertificateInfo{
			Name:         cert.Subject.CommonName,
			SerialNumber: cert.SerialNumber.Text(16),
			CreationDate: cert.NotBefore,
			ExpiryDate:   cert.NotAfter,
		}

		// Calculate SHA1 fingerprint
		sha1Sum := sha1Fingerprint(certData)
		info.SHA1 = sha1Sum

		result = append(result, info)
	}

	return result
}

// sha1Fingerprint calculates the SHA1 fingerprint of certificate data
func sha1Fingerprint(data []byte) string {
	h := sha1.Sum(data)
	return fmt.Sprintf("%X", h)
}

// parseAPKFromPath parses APK metadata from a file path (used by SmartUpload).
func parseAPKFromPath(path, origFilename string, fileSize int64) (gin.H, error) {
	var manifestBuf bytes.Buffer
	xmlEnc := xml.NewEncoder(&manifestBuf)
	xmlEnc.Indent("", "  ")
	zipErr, _, manErr := apkparser.ParseApk(path, xmlEnc)
	if zipErr != nil {
		return nil, fmt.Errorf("failed to open APK: %w", zipErr)
	}
	if manErr != nil {
		return nil, fmt.Errorf("failed to parse AndroidManifest.xml: %w", manErr)
	}
	_ = xmlEnc.Flush()

	pkgName, verName, verCode := "", "", 0
	minSDK, targetSDK := "", ""
	appLabel, iconRef := "", ""

	dec := xml.NewDecoder(bytes.NewReader(manifestBuf.Bytes()))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			if name == "manifest" {
				for _, a := range t.Attr {
					if a.Name.Local == "package" {
						pkgName = a.Value
					}
					if a.Name.Local == "versionName" {
						verName = a.Value
					}
					if a.Name.Local == "versionCode" {
						if n, err := strconv.Atoi(a.Value); err == nil {
							verCode = n
						}
					}
				}
			} else if name == "uses-sdk" {
				for _, a := range t.Attr {
					if a.Name.Local == "minSdkVersion" {
						minSDK = a.Value
					}
					if a.Name.Local == "targetSdkVersion" {
						targetSDK = a.Value
					}
				}
			} else if name == "application" {
				for _, a := range t.Attr {
					if a.Name.Local == "label" && appLabel == "" {
						appLabel = a.Value
					}
					if a.Name.Local == "icon" && iconRef == "" {
						iconRef = a.Value
					}
					if a.Name.Local == "roundIcon" && iconRef == "" {
						iconRef = a.Value
					}
				}
			}
		}
	}

	res := gin.H{
		"platform":     "android",
		"package_name": pkgName,
		"version_name": verName,
		"version_code": verCode,
		"app_name":     appLabel,
		"file_size":    fileSize,
		"file_name":    origFilename,
	}
	if minSDK != "" {
		res["min_sdk"] = fmt.Sprintf("Android %s", minSDK)
	}
	if targetSDK != "" {
		res["target_sdk"] = fmt.Sprintf("Android %s", targetSDK)
	}
	if iconData, _ := extractAPKIconFromPathCandidates(path, iconRef); iconData != "" {
		res["icon_base64"] = iconData
	} else if iconData := extractAPKIconHeuristic(path); iconData != "" {
		res["icon_base64"] = iconData
	}
	if res["app_name"] == "" && pkgName != "" {
		parts := strings.Split(pkgName, ".")
		res["app_name"] = strings.Title(parts[len(parts)-1])
	}
	return res, nil
}

// parseIPAFromPath parses IPA metadata from a file path (used by SmartUpload).
func parseIPAFromPath(path, origFilename string, fileSize int64) (gin.H, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to read IPA: %w", err)
	}

	var plistFile *zip.File
	var appDir string
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "Payload/") && strings.HasSuffix(f.Name, ".app/Info.plist") {
			plistFile = f
			appDir = filepath.Dir(f.Name)
			break
		}
	}
	if plistFile == nil {
		return nil, fmt.Errorf("Info.plist not found")
	}

	rc, err := plistFile.Open()
	if err != nil {
		return nil, err
	}
	plistData, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if _, err := plist.Unmarshal(plistData, &m); err != nil {
		return nil, fmt.Errorf("failed to parse Info.plist: %w", err)
	}

	bundleID := getPlistString(m, "CFBundleIdentifier")
	version := getPlistString(m, "CFBundleShortVersionString")
	build := getPlistString(m, "CFBundleVersion")
	appName := getPlistString(m, "CFBundleDisplayName")
	if appName == "" {
		appName = getPlistString(m, "CFBundleName")
	}
	minOS := getPlistString(m, "MinimumOSVersion")

	res := gin.H{
		"platform":  "ios",
		"bundle_id": bundleID,
		"version":   version,
		"build":     build,
		"app_name":  appName,
		"min_os":    fmt.Sprintf("iOS %s", minOS),
		"file_size": fileSize,
		"file_name": origFilename,
	}

	if icon := extractIPAIconFromAssetsCar(zr, appDir, m); icon != "" {
		res["icon_base64"] = icon
	} else if icon := extractIPAIcon(zr, appDir, m); icon != "" {
		res["icon_base64"] = icon
	}

	if profile := extractMobileProvision(zr, appDir); profile != nil {
		res["provisioning_profile"] = profile
	}

	return res, nil
}
