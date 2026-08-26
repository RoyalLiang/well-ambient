import { mount } from 'svelte';
import './app.css';
import './styles/modern-admin-tokens.css';
import AdminDataListPreview from './components/prototype/AdminDataListPreview.svelte';

mount(AdminDataListPreview, {
  target: document.getElementById('admin-data-list-preview')!
});
