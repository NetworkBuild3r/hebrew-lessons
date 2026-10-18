# Toolchain stage. esbuild and typescript are devDependencies, so the runtime
# stage below still installs with npm ci --omit=dev. Typecheck fails the image.
FROM node:22-alpine AS player
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY web ./web
RUN npm run typecheck && npm run build:player && test -s shared/dist/player.js

FROM node:22-alpine
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev
COPY . .
COPY --from=player /app/shared/dist ./shared/dist
RUN mkdir -p /app/data && chown -R node:node /app/data
USER node
EXPOSE 8080 8787
CMD ["node", "server/start.js"]
