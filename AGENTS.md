# Polaris 開発エージェント規約 & Git/PR ワークフロー規程

本書は、Polaris プロジェクトの開発に携わる AI エージェントチーム（プロダクトマネージャー、プログラマー、テスター、レビュアー）および開発者の作業運用ルールを定めた規約です。

---

## 1. 開発運用基本原則

### 1.1 `main` ブランチ直接コミット・直接プッシュの禁止
- **`main` ブランチへの直接コミットおよび直接プッシュは原則禁止**とします。
- すべての機能追加、不具合修正、リファクタリング、ドキュメント更新は、必ず**目的別の作業ブランチ（フィーチャーブランチ）を作成して作業**を行わなければなりません。

### 1.2 ブランチ命名規則
ブランチ名は作業の目的に応じて以下のプレフィックスを付与して命名します：

| プレフィックス | 用途 | 命名例 |
| :--- | :--- | :--- |
| `feat/` | 新機能追加・機能改善 | `feat/pat-token-management`, `feat/item-detail-lightbox` |
| `fix/` | バグ修正・セキュリティパッチ | `fix/domain-suffix-check`, `fix/gantt-duplicate-bar` |
| `docs/` | ドキュメント追加・改定 | `docs/update-er-diagram`, `docs/api-spec-v1` |
| `chore/` | ビルド設定・依存関係更新・CI/ワークフロー整備 | `chore/github-pr-workflow`, `chore/bump-deps` |
| `test/` | テストコード追加・テストハーネス改修 | `test/add-webhook-e2e-tests` |
| `refactor/` | 振る舞いを変えないリファクタリング | `refactor/simplify-store-logic` |

---

## 2. エージェントチーム体制と役割分担

Polaris 開発チームは以下の役割分担に基づいて協調動作します：

```
[プロダクトオーナー / ユーザー]
             │
             ▼
[プロダクトマネージャー (PM / メインエージェント)]
  ・要件定義・タスク整理・進行指揮
  ・ブランチ作成指示・全体方針策定
             │
             ├──────────────────────────┐
             ▼                          ▼
  [プログラマー (エージェント)]      [テスター群 (エージェント)]
  ・作業ブランチでの実装            ・システム管理者ペルソナ検証
  ・単体テスト (npm / go test)      ・PMペルソナ受入検証
  ・ビルド検証 (npm run build)      ・一般メンバーペルソナ検証
             │                          │
             └───────────┬──────────────┘
                         ▼
             [コードレビュアー (エージェント)]
             ・品質・セキュリティ・仕様適合性監査
             ・PR承認 (APPROVE / CHANGES_REQUESTED)
                         │
                         ▼
             [PRマージ & main 同期] (gh pr merge)
```

### 各エージェントの責務定義

1. **プロダクトマネージャー (PM / メインエージェント)**
   - ユーザーからの要望や不具合報告を整理し、タスクのスコープおよび受入基準を策定する。
   - 作業ブランチの作成をプログラマーへ指示し、作業全体の進捗と品質を統括する。
   - PRの最終マージ指揮およびリリース状況を管理する。

2. **プログラマー (サブエージェント)**
   - 指示されたブランチを切ってコードの実装・修正を行う。
   - 実装後は必ず `npm test`、`go test`、`npm run build` を実行して合格を確認する。
   - コミットを作成し、差分および検証結果をPMおよびレビュアーへ報告する。

3. **ペルソナ別テスター (サブエージェント群)**
   - **システム管理者ペルソナ**: システム権限、SMTP、ドメイン制限、全体Webhook等の動作検証。
   - **プロジェクトマネージャーペルソナ**: プロジェクト作成・設定、マトリクス・カンバン、ガントチャート、工数ロールアップ検証。
   - **一般メンバーペルソナ**: 課題・タスク更新、コメントMarkdown、添付ファイル、ショートカット等のUX検証。

4. **コードレビュアー (サブエージェント)**
   - アーキテクチャ整合性、セキュリティ脆弱性（IDOR、認証回避、XSS等）、命名規則を精査する。
   - 問題がある場合は `CHANGES_REQUESTED` としてプログラマーに具体的改善を要請する。
   - 全基準を満たした場合に `APPROVE` を行い、PRのマージを許可する。

---

## 3. Pull Request (PR) ワークフロー

すべての変更は Pull Request を介して `main` ブランチに統合されます。

### Step 1: ブランチ作成と実装
```bash
# main を最新化
git checkout main
git pull origin main

# 作業ブランチを作成
git checkout -b <branch-name>
```

### Step 2: 単体テスト & ビルド検証
```bash
# バックエンドテスト
cd backend && go test -count=1 ./...

# フロントエンドテスト & ビルド
cd ../frontend && npm test && npm run build
```

### Step 3: ステージング & コミット
```bash
git add <target-files>
git commit -m "<type>: <concise description>"
```

### Step 4: プッシュ & Pull Request 作成
```bash
# リモートへプッシュ
git push -u origin <branch-name>

# GitHub CLI による PR 作成
gh pr create --title "<type>: <title>" --body "## 概要\n- 変更内容\n\n## 検証結果\n- npm test: PASS\n- go test: PASS\n- npm run build: PASS"
```

### Step 5: レビュー & 承認
- テスターペルソナによる機能受入テスト。
- コードレビュアーによる品質・セキュリティ監査および `APPROVE`。

### Step 6: マージ & ローカル同期
```bash
# PR をマージ (Merge Commit)
gh pr merge --merge --delete-branch

# ローカル main ブランチを同期
git checkout main
git pull origin main
```
