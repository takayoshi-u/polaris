#!/usr/bin/env bash
# ==============================================================================
# Polaris - クイックスタート起動スクリプト (quickstart.sh)
# ==============================================================================
# 実行権限を付与して実行してください:
#   chmod +x quickstart.sh
#   ./quickstart.sh
# ==============================================================================

set -euo pipefail

# カラー設定
BLUE='\033[1;34m'
GREEN='\033[1;32m'
YELLOW='\033[1;33m'
RED='\033[1;31m'
CYAN='\033[1;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}"
cat << 'EOF'
  _____       _            _     
 |  __ \     | |          (_)    
 | |__) |___ | | __ _ _ __ _ ___ 
 |  ___// _ \| |/ _` | '__| / __|
 | |   | (_) | | (_| | |  | \__ \
 |_|    \___/|_|\__,_|_|  |_|___/
                                 
   Polaris Quick Start Launcher
EOF
echo -e "${NC}"

# 1. 依存ツールの確認
echo -e "${CYAN}[1/4] 前提条件の確認中...${NC}"

if ! command -v docker >/dev/null 2>&1; then
    echo -e "${RED}エラー: Docker がインストールされていません。${NC}"
    echo "Docker をインストールしてから再実行してください: https://docs.docker.com/get-docker/"
    exit 1
fi

DOCKER_COMPOSE_CMD=""
if docker compose version >/dev/null 2>&1; then
    DOCKER_COMPOSE_CMD="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
    DOCKER_COMPOSE_CMD="docker-compose"
else
    echo -e "${RED}エラー: Docker Compose がインストールされていません。${NC}"
    echo "Docker Compose をインストールしてください: https://docs.docker.com/compose/install/"
    exit 1
fi
echo -e "${GREEN}✓ Docker & Docker Compose を検出しました。${NC}"

# 2. 環境変数設定 (.env)
echo -e "${CYAN}[2/4] 設定ファイルの準備中...${NC}"
if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        cp .env.example .env
        echo -e "${GREEN}✓ .env.example から .env を生成しました。${NC}"
    else
        touch .env
        echo -e "${YELLOW}! .env を新規作成しました。${NC}"
    fi
else
    echo -e "${GREEN}✓ 既存の .env を使用します。${NC}"
fi

# 3. コンテナのビルドと起動
echo -e "${CYAN}[3/4] Docker コンテナをビルド・起動中 (PostgreSQL, Go API, Nuxt 3 Web, Mailpit)...${NC}"
$DOCKER_COMPOSE_CMD up -d --build

# 4. 起動待機とヘルスチェック
echo -e "${CYAN}[4/4] サービスの起動を確認中...${NC}"

API_READY=false
for i in {1..30}; do
    if curl -s -f http://localhost:8080/api/v1/health >/dev/null 2>&1 || curl -s http://localhost:8080/ >/dev/null 2>&1; then
        API_READY=true
        break
    fi
    sleep 1
done

echo ""
echo -e "${GREEN}================================================================${NC}"
echo -e "${GREEN}🚀 Polaris の起動が完了しました！${NC}"
echo -e "${GREEN}================================================================${NC}"
echo ""
echo -e "ブラウザで以下のURLにアクセスしてください："
echo ""
echo -e "  🌐 ${BLUE}フロントエンド Web アプリ${NC} : ${GREEN}http://localhost:3000${NC}"
echo -e "  ⚙️  ${BLUE}初回セットアップ画面${NC}     : ${YELLOW}http://localhost:3000/setup${NC}"
echo -e "  📬 ${BLUE}開発用メール確認 UI (Mailpit)${NC}: ${CYAN}http://localhost:8025${NC}"
echo -e "  🔌 ${BLUE}バックエンド REST API${NC}       : http://localhost:8080/api/v1"
echo ""
echo -e "${YELLOW}【はじめの一歩】${NC}"
echo "1. ブラウザで http://localhost:3000/setup にアクセスし、初期管理者アカウントを登録します。"
echo "2. ログイン後、「＋ 新規プロジェクト」ボタンからプロジェクトを作成します。"
echo "3. 課題スイムレーン×ステータスカンバンやガントチャートをお試しいただけます。"
echo "4. ユーザー招待メールは Mailpit (http://localhost:8025) で即座に確認できます。"
echo ""
echo -e "停止コマンド: ${CYAN}$DOCKER_COMPOSE_CMD down${NC}"
echo -e "ログ確認コマンド: ${CYAN}$DOCKER_COMPOSE_CMD logs -f${NC}"
echo -e "${GREEN}================================================================${NC}"

