# Build the API-connected static app. This is not a production qualification.
ARG NODE_IMAGE=node:22-alpine
FROM ${NODE_IMAGE} AS build
WORKDIR /src
COPY WebApp/package.json WebApp/package-lock.json ./
RUN npm ci --ignore-scripts
COPY WebApp/ ./
ARG VITE_API_BASE_URL=https://api.example.invalid
ENV VITE_API_MODE=api
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
RUN npm run build
# Review release must record immutable image digests before live deployment.
FROM nginxinc/nginx-unprivileged:stable-alpine
COPY --from=build /src/dist/ /usr/share/nginx/html/
COPY WebApp/deploy/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 8080
