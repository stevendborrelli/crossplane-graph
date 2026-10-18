import { mount } from 'svelte';
import '@fontsource-variable/league-spartan';
import '@fontsource-variable/jetbrains-mono';
import './app.css';
import App from './App.svelte';

mount(App, { target: document.getElementById('app') });
