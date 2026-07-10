import { mount } from 'svelte'
import './styles/modern-admin-tokens.css'
import FunctionalAdminPreview from './components/prototype/FunctionalAdminPreview.svelte'

const target = document.getElementById('prototype')

if (!target) {
  throw new Error('Missing prototype mount node')
}

const app = mount(FunctionalAdminPreview, {
  target,
})

export default app
