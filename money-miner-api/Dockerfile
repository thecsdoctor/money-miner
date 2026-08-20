# Optional all-in-one image for hosting the editor yourself.
# The dev compose stack in this directory uses the upstream
# swaggerapi/swagger-editor image with the spec bind-mounted instead.
FROM swaggerapi/swagger-editor:v5.8.5@sha256:db2dff1f0a8865d23eb6cbd4b52d68e85e76ad5b391a7c49e374f1cb237ff5fb
COPY openapi.yaml /usr/share/nginx/html/openapi.yaml
ENV URL=/openapi.yaml
