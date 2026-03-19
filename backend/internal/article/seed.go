package article

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ai-api-portal/backend/internal/model"
)

// SeedLegacyArticles inserts the 5 hardcoded blog articles from the frontend
// into the articles table. It is idempotent: articles whose legacy_id already
// exists are skipped.
func SeedLegacyArticles(db *sql.DB) error {
	svc := NewService(db)
	ctx := context.Background()

	articles := legacyArticles()
	for _, a := range articles {
		exists, err := legacyIDExists(db, *a.LegacyID)
		if err != nil {
			return fmt.Errorf("check legacy_id %d: %w", *a.LegacyID, err)
		}
		if exists {
			continue
		}
		if err := svc.CreateArticle(ctx, &a); err != nil {
			return fmt.Errorf("seed article %q (legacy_id=%d): %w", a.Title, *a.LegacyID, err)
		}
	}

	return nil
}

func legacyIDExists(db *sql.DB, legacyID int64) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(1) FROM articles WHERE legacy_id = ?;`, legacyID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query legacy_id existence: %w", err)
	}
	return count > 0, nil
}

func legacyArticles() []model.Article {
	return []model.Article{
		{
			LegacyID:      intPtr(1),
			Slug:          "gpt4o-vs-claude35",
			Title:         "GPT-4o 与 Claude 3.5 深度对比评测",
			Excerpt:       strPtr("从代码能力、推理能力、创意写作等多个维度对比两大主流AI模型的实际表现。"),
			CoverImageURL: strPtr("https://picsum.photos/seed/blog1/800/400"),
			Tag:           strPtr("AI评测"),
			ReadTime:      strPtr("10 min read"),
			MDXBody: `## 前言

在AI模型快速迭代的今天，GPT-4o 和 Claude 3.5 Sonnet 无疑是当前最受关注的两大主流模型。本文将从多个维度对它们进行深度对比评测。

## 测试环境

- 测试时间：2025年1月
- 测试场景：代码生成、逻辑推理、创意写作、多语言处理
- 评测标准：准确性、响应速度、成本效益

## 代码能力对比

### GPT-4o 表现
GPT-4o 在代码补全和调试方面表现出色，特别是在处理复杂算法时能够给出清晰的解释。

### Claude 3.5 Sonnet 表现
Claude 3.5 在代码审查和重构建议方面更为细致，能够发现潜在的边界情况问题。

## 推理能力

两款模型在复杂推理任务上都有不错的表现，但 Claude 3.5 在需要多步骤推理的任务中表现略胜一筹。

## 成本分析

从API调用成本来看，两者的定价策略各有优势，需要根据具体使用场景选择。

## 总结

选择哪个模型取决于你的具体需求。如果你需要强大的代码能力，GPT-4o 是不错的选择；如果你更看重推理和分析能力，Claude 3.5 可能更适合。`,
			Status:      statusPublished,
			PublishedAt: parseDate("2025-01-15"),
		},
		{
			LegacyID:      intPtr(2),
			Slug:          "cursor-tips",
			Title:         "Cursor 编辑器高效使用技巧",
			Excerpt:       strPtr("分享我在使用 Cursor 进行AI辅助编程时总结的一些实用技巧和最佳实践。"),
			CoverImageURL: strPtr("https://picsum.photos/seed/blog2/800/400"),
			Tag:           strPtr("工具技巧"),
			ReadTime:      strPtr("8 min read"),
			MDXBody: `## 为什么选择 Cursor

Cursor 是一款基于 VSCode 的 AI 辅助编程工具，它集成了强大的 AI 能力，能够显著提升开发效率。

## 核心功能介绍

### 1. AI 对话
使用 Cmd+L 快速唤起 AI 对话窗口，可以直接询问代码相关问题。

### 2. 代码补全
Tab 键接受 AI 建议，支持多行代码补全。

### 3. 代码重构
选中代码后使用 Cmd+K 进行智能重构。

## 最佳实践

1. **善用上下文**：让 AI 了解你的项目结构
2. **渐进式开发**：先写核心逻辑，再让 AI 补充细节
3. **代码审查**：让 AI 帮你检查代码质量

## 快捷键总结

- Cmd+L：打开 AI 对话
- Cmd+K：内联编辑
- Tab：接受建议
- Cmd+Shift+L：解释代码`,
			Status:      statusPublished,
			PublishedAt: parseDate("2024-12-20"),
		},
		{
			LegacyID:      intPtr(3),
			Slug:          "deepseek-vs-qwen",
			Title:         "国产大模型横评：DeepSeek vs Qwen",
			Excerpt:       strPtr("对比评测 DeepSeek 和通义千问在中文场景下的表现，帮你选择合适的模型。"),
			CoverImageURL: strPtr("https://picsum.photos/seed/blog3/800/400"),
			Tag:           strPtr("AI评测"),
			ReadTime:      strPtr("12 min read"),
			MDXBody: `## 评测背景

国产大模型发展迅速，DeepSeek 和通义千问（Qwen）是目前最受关注的两款产品。本文将从中文处理能力、代码能力、性价比等维度进行对比。

## 中文理解能力

### DeepSeek
- 古文理解能力较强
- 成语运用准确
- 中文创意写作流畅

### 通义千问
- 中文常识知识丰富
- 对中国文化背景理解深入
- 商务文案生成专业

## 代码能力

两款模型都支持主流编程语言，但在细节表现上各有千秋。

## API 接入

两款产品都提供了完善的 API 接口，接入文档清晰。

## 价格对比

DeepSeek 的定价更具竞争力，适合大规模调用的场景。

## 选择建议

- 注重性价比：选择 DeepSeek
- 需要丰富中文知识：选择通义千问
- 两者都是优秀的国产模型`,
			Status:      statusPublished,
			PublishedAt: parseDate("2024-12-15"),
		},
		{
			LegacyID:      intPtr(4),
			Slug:          "deepseek-vs-qwen-2",
			Title:         "国产大模型横评：DeepSeek vs Qwen",
			Excerpt:       strPtr("对比评测 DeepSeek 和通义千问在中文场景下的表现，帮你选择合适的模型。"),
			CoverImageURL: strPtr("https://picsum.photos/seed/blog4/800/400"),
			Tag:           strPtr("AI评测"),
			ReadTime:      strPtr("12 min read"),
			MDXBody: `## 评测背景

国产大模型发展迅速，DeepSeek 和通义千问（Qwen）是目前最受关注的两款产品。本文将从中文处理能力、代码能力、性价比等维度进行对比。

## 中文理解能力

### DeepSeek
- 古文理解能力较强
- 成语运用准确
- 中文创意写作流畅

### 通义千问
- 中文常识知识丰富
- 对中国文化背景理解深入
- 商务文案生成专业

## 代码能力

两款模型都支持主流编程语言，但在细节表现上各有千秋。

## API 接入

两款产品都提供了完善的 API 接口，接入文档清晰。

## 价格对比

DeepSeek 的定价更具竞争力，适合大规模调用的场景。

## 选择建议

- 注重性价比：选择 DeepSeek
- 需要丰富中文知识：选择通义千问
- 两者都是优秀的国产模型`,
			Status:      statusPublished,
			PublishedAt: parseDate("2024-12-15"),
		},
		{
			LegacyID:      intPtr(5),
			Slug:          "api-key-security",
			Title:         "API Key 安全管理最佳实践",
			Excerpt:       strPtr("如何安全地管理和存储你的 API 密钥，避免泄露和滥用。"),
			CoverImageURL: strPtr("https://picsum.photos/seed/blog5/800/400"),
			Tag:           strPtr("安全"),
			ReadTime:      strPtr("4 min read"),
			MDXBody: `## 为什么 API Key 安全很重要

API Key 泄露可能导致：
- 账户被滥用产生高额费用
- 敏感数据泄露
- 服务被恶意调用

## 安全管理原则

### 1. 永远不要硬编码
不要将 API Key 直接写在代码中，应该使用环境变量。

### 2. 使用 .env 文件
创建 .env 文件存储密钥，并将其加入 .gitignore。

### 3. 定期轮换密钥
建议每隔 3-6 个月更换一次 API Key。

### 4. 设置使用限制
为不同的应用创建不同的 Key，并设置合理的使用限额。

## 代码示例

` + "```" + `bash
# .env 文件
API_KEY=your_secret_key_here
` + "```" + `

` + "```" + `javascript
// 读取环境变量
const apiKey = process.env.API_KEY;
` + "```" + `

## 泄露后的应对措施

1. 立即撤销泄露的 Key
2. 检查账户使用记录
3. 创建新的 Key
4. 更新所有使用该 Key 的应用`,
			Status:      statusPublished,
			PublishedAt: parseDate("2024-12-01"),
		},
	}
}

func intPtr(v int64) *int64 { return &v }

func strPtr(v string) *string { return &v }

func parseDate(s string) *time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}
