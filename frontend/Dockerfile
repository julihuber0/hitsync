FROM node:22-alpine AS build
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /src/dist /usr/share/nginx/html
# Templated (via the base image's built-in envsubst entrypoint script) so
# CSP can reference LIVEKIT_DOMAIN/APP_DOMAIN without baking them into the
# image at build time (§15.1: the image stays environment-independent).
COPY nginx.conf /etc/nginx/templates/default.conf.template
EXPOSE 80
