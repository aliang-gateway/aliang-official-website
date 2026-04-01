# Stage 1: Install dependencies
FROM node:25-alpine3.22 AS deps
WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci

# Stage 2: Build the application
FROM node:25-alpine3.22 AS builder
WORKDIR /app

ARG CDN_URL=""
ENV NEXT_PUBLIC_CDN_URL=$CDN_URL

COPY --from=deps /app/node_modules ./node_modules
COPY . .

RUN npm run build

# Stage 3: Production image
FROM node:25-alpine3.22 AS runner
WORKDIR /app

ENV NODE_ENV=production

# Runtime server code reads API_BASE_URL directly. Keep NEXT_PUBLIC_API_BASE_URL as
# a compatibility fallback for older env wiring.
ARG API_BASE_URL="http://aliang-website-backend:8081"
ENV API_BASE_URL=$API_BASE_URL
ENV NEXT_PUBLIC_API_BASE_URL=$API_BASE_URL

RUN addgroup --system --gid 1001 nodejs \
    && adduser --system --uid 1001 nextjs

# 安装 coscli 用于上传静态资源到腾讯云 COS
RUN wget -O /usr/local/bin/coscli https://cosbrowser.cloud.tencent.com/software/coscli/coscli-linux-amd64 \
    && chmod +x /usr/local/bin/coscli

COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY scripts/upload-to-cdn.sh /app/scripts/upload-to-cdn.sh

USER nextjs

EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
