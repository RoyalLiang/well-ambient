import './app.css';
import { mount } from 'svelte';
import SolutionSavePreview from './components/prototype/SolutionSavePreview.svelte';

mount(SolutionSavePreview, { target: document.getElementById('app')! });
