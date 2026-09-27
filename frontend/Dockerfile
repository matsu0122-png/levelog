FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
# Baked into the static bundle at build time. Leave unset (default "") to
# use the same-origin /api/ proxy configured in nginx.conf; set to an
# absolute URL only if the API is served from a different origin.
ARG VITE_API_BASE_URL=""
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
RUN npm run build

# nginx-unprivileged already listens on 8080 and runs as a non-root user
# out of the box, so no extra user/permission setup is needed here.
FROM nginxinc/nginx-unprivileged:1.30-alpine
# Pull in Alpine security fixes that landed after the base image was last
# rebuilt (e.g. libexpat CVE-2026-93990, caught by CI's Trivy scan). Needs
# root for apk; switch back to the image's own non-root user (101) after.
USER root
RUN apk upgrade --no-cache
USER 101
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY nginx-locations.conf /etc/nginx/snippets/nginx-locations.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 8080 8443
# /healthz (not /health/ or /) deliberately: it's answered by Nginx alone,
# so this reflects Nginx's own liveness rather than the api container's —
# see nginx.conf.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://127.0.0.1:8080/healthz || exit 1
