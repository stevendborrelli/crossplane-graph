import { getContext, setContext } from 'svelte';

const KEY = Symbol('xplanet.scene');

export function setScene(scene) {
  setContext(KEY, scene);
}

export function getScene() {
  const scene = getContext(KEY);
  if (!scene) {
    throw new Error('This component must be used inside a <Scene>.');
  }
  return scene;
}
