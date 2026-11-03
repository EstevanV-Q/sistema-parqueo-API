// Package api expone la especificación OpenAPI y una UI de Swagger embebidas
// en el binario para que la documentación viaje con el servicio.
package api

import _ "embed"

// OpenAPISpec es el contrato OpenAPI 3.0.3 del backend.
//
//go:embed openapi.yaml
var OpenAPISpec []byte

// SwaggerHTML es una página mínima que carga Swagger UI desde CDN y apunta al
// spec servido por el propio backend en /swagger/openapi.yaml.
const SwaggerHTML = `<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Sistema de Parqueo API - Swagger</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body { margin: 0; background: #fafafa; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({
        url: "/swagger/openapi.yaml",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis],
        layout: "BaseLayout",
      });
    };
  </script>
</body>
</html>`
