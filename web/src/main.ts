import { mount } from 'svelte'
import './styles/modern-admin-tokens.css'
import './app.css'
import App from './App.svelte'

const app = mount(App, {
  target: document.getElementById('app')!,
})

export default app
