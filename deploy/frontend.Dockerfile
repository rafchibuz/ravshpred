FROM node:22-alpine AS build
WORKDIR /app
ARG VITE_QA_MODE=0
ENV VITE_QA_MODE=$VITE_QA_MODE
RUN corepack enable
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY index.html ./
COPY src ./src
COPY assets ./assets
RUN pnpm build

FROM caddy:2.10-alpine
COPY deploy/Caddyfile /etc/caddy/Caddyfile
COPY --from=build /app/dist /srv
EXPOSE 80
