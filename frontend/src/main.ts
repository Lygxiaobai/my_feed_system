import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './style.css'
import { installAnalyticsFlush, track } from './analytics/track'
import App from './App.vue'
import router from './router'
import { useThemeStore } from './stores/theme'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)
useThemeStore(pinia).start()
installAnalyticsFlush()

router.afterEach((to) => {
  track('page_view', {
    name: typeof to.name === 'string' ? to.name : to.path,
  })
})

app.mount('#app')
