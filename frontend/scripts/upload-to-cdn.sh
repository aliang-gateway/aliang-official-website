#!/bin/bash
# 一键上传构建产物到腾讯云 COS
# 用法: docker exec <container> bash /app/scripts/upload-to-cdn.sh

set -e

# === 配置 ===
COS_BUCKET="aliang-1305838434"
COS_REGION="ap-nanjing"

# 从环境变量读取密钥（运行时传入，不硬编码）
: "${COS_SECRET_ID:?错误: 请设置环境变量 COS_SECRET_ID}"
: "${COS_SECRET_KEY:?错误: 请设置环境变量 COS_SECRET_KEY}"

# === 生成 coscli 配置文件 ===
COS_CONFIG="${HOME}/.cos.yaml"
cat > "$COS_CONFIG" <<EOF
cos:
  base:
    secretid: ${COS_SECRET_ID}
    secretkey: ${COS_SECRET_KEY}
    sessiontoken: ""
    protocol: https
  buckets:
    - name: ${COS_BUCKET}
      alias: cdn
      region: ${COS_REGION}
      endpoint: cos.${COS_REGION}.myqcloud.com
      ofs: false
EOF

echo "=> 上传 public/ 到 COS ..."
coscli cp -r /app/public/ "cos://${COS_BUCKET}/"

echo "=> 上传 .next/static/ 到 COS ..."
coscli cp -r /app/.next/static/ "cos://${COS_BUCKET}/_next/static/"

echo "=> 上传完成!"
echo "CDN URL: https://${COS_BUCKET}.cos.${COS_REGION}.myqcloud.com"
