import { createRouter, createWebHashHistory } from 'vue-router'
import AppsPage from './pages/AppsPage.vue'
import UploadPage from './pages/UploadPage.vue'
import StatsPage from './pages/StatsPage.vue'
import EventsPage from './pages/EventsPage.vue'
import UdidPage from './pages/UdidPage.vue'
import ExportPage from './pages/ExportPage.vue'
import SettingsPage from './pages/SettingsPage.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', redirect: '/apps' },
    { path: '/apps', component: AppsPage },
    { path: '/upload', component: UploadPage },
    { path: '/stats', component: StatsPage },
    { path: '/stats/:variantId', component: StatsPage, props: true },
    { path: '/events', component: EventsPage },
    { path: '/events/:variantId', component: EventsPage, props: true },
    { path: '/udid', component: UdidPage },
    { path: '/export', component: ExportPage },
    { path: '/settings', component: SettingsPage },
  ],
})

export default router
