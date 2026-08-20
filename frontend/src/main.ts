import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './style.css'
import { installAnalyticsFlush, track } from './analytics/track'
import App from './App.vue'
import router from './router'

const app = createApp(App)
app.use(createPinia())
app.use(router)
installAnalyticsFlush()

router.afterEach((to) => {
  track('page_view', {
    name: typeof to.name === 'string' ? to.name : to.path,
  })
})

app.mount('#app')
