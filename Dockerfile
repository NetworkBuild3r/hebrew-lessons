# syntax=docker/dockerfile:1

# Production dependencies. sharp's musl builds match this base, so the
# runtime stage can copy node_modules without a rebuild.
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --omit=dev

# No build stage: package.json has no compile or asset script
# (start, test, and the operator CLIs only).

FROM node:22-alpine AS runtime
WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY package.json ./
COPY server ./server
COPY mcp ./mcp
COPY shared ./shared
COPY worksheets ./worksheets
COPY login ./login
COPY scripts ./scripts
COPY icon.svg manifest.webmanifest ./
# Lesson upsert shells out to scripts/validate.js. The rest of this skill
# (SKILL.md, recipes.md, schema.md, example.md) sits beside it.
COPY .cursor/skills/hebrew-worksheet ./.cursor/skills/hebrew-worksheet
# server/plans/validate.js and hebrew-checks.js read this file by path.
COPY docs/decisions/ADR-0003-assets/lesson-plan.schema.json \
    ./docs/decisions/ADR-0003-assets/lesson-plan.schema.json

RUN mkdir -p /app/data && chown -R node:node /app/data
USER node
EXPOSE 8080 8787
CMD ["node", "server/start.js"]
