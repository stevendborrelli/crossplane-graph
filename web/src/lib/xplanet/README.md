# XPlanet primitives

`Scene.svelte`, `NodeShell.svelte`, `scene-context.js` and `geometry.js` are
copied from Steven Borrelli's XPlanet site
(`src/components/primitives/` and `src/lib/motion/geometry.js`), and licensed
here under this repository's Apache 2.0 license by their author.

The only change is Scene's import of `geometry.js`, which sits beside it here.
Keep them otherwise as they are in XPlanet, so a fix in one can be copied to
the other. The rails between boxes are drawn by `../../Wires.svelte` rather than
XPlanet's `Edge`, routed for a graph laid out in waves.
