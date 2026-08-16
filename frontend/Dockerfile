FROM node:22-alpine AS builder

WORKDIR /app
RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY packages ./packages
RUN pnpm install --frozen-lockfile

# Container builds must be reproducible without copying developer .env files.
# These values are public browser build settings, not runtime secrets.
ARG VITE_BASE_URL=/
ARG VITE_ICON_PREFIX=icon
ARG VITE_ICON_LOCAL_PREFIX=local-icon
ARG VITE_AUTH_ROUTE_MODE=dynamic
ARG VITE_ROUTE_HOME=dashboard_workbench
ARG VITE_MENU_ICON=mdi:menu
ARG VITE_ENABLE_THINGSVIS_COMPAT=N
ENV VITE_BASE_URL=${VITE_BASE_URL} \
    VITE_ICON_PREFIX=${VITE_ICON_PREFIX} \
    VITE_ICON_LOCAL_PREFIX=${VITE_ICON_LOCAL_PREFIX} \
    VITE_AUTH_ROUTE_MODE=${VITE_AUTH_ROUTE_MODE} \
    VITE_ROUTE_HOME=${VITE_ROUTE_HOME} \
    VITE_MENU_ICON=${VITE_MENU_ICON} \
    VITE_ENABLE_THINGSVIS_COMPAT=${VITE_ENABLE_THINGSVIS_COMPAT}

COPY . .
RUN pnpm build

FROM nginx:1.31-alpine

COPY --from=builder /app/dist /usr/share/nginx/html/
COPY nginx.conf /etc/nginx/nginx.conf

# The upstream nginx image drops privileges to this account after binding.
# Compose removes ambient capabilities and prevents privilege escalation.
EXPOSE 8080
ENTRYPOINT nginx -g "daemon off;"
