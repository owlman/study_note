# Telemetry Content

This directory contains the templates, styles, scripts, and images used in the
telemetry services. Scripts and styles are transformed and minified by the
generator in [content.go](./content.go).

## Scripts & Styles

The generator command will look for entrypoint scripts and styles, i.e. files
that are not prefixed with an underscore, and minfiy their contents. TypeScript
files are also transformed into JavaScript. See
[devtools/cmd/esbuild](../devtools/cmd/esbuild/main.go) for more information.

## Templates

Use the .html extension to create a new route, or put an index.html file in a
directory with the desired path. Partial templates with the extension .tmpl in
the same directory as the requested page are included in the html/template
execution step to allow for sharing and composing multiple templates. See
[internal/content](../internal/content/content.go) for more information.
