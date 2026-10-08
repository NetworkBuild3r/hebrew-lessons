FROM node:22-alpine
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev
COPY . .
RUN mkdir -p /app/data && chown -R node:node /app/data
USER node
EXPOSE 8080 8787
CMD ["node", "server/start.js"]
