# Build stage
FROM node:20-alpine AS build
WORKDIR /app
COPY webui/package.json webui/package-lock.json* ./webui/
RUN cd webui && npm install --no-audit --no-fund || (npm init -y && npm install --no-audit --no-fund)
COPY webui ./webui
RUN cd webui && npm run build

# Runtime stage
FROM nginx:1.25-alpine
COPY --from=build /app/webui/dist /usr/share/nginx/html
COPY webui/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 8080
CMD ["nginx","-g","daemon off;"]
