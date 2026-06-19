package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const systemPrompt = `你是一位专门服务于中国高中（高级中学）教学环境的文件分类专家。
请根据给定的【文件名】或【文件正文片段】，判断它属于以下唯一类别之一：
数学, 语文, 英语, 物理, 化学, 生物, 无关文件, 无法确定。

判定标准必须紧扣中国高中课程标准（人教版/统编版教材内容）。例如：
- 出现"三角函数、向量、导数" → 数学
- 出现"文言文、古诗词、修辞手法" → 语文
- 出现"牛顿定律、电磁感应、受力分析" → 物理
- 出现"细胞分裂、基因、光合作用" → 生物
- 出现"安装包、注册表、游戏启动器、显卡驱动" → 无关文件
- 若内容过于简短或无法匹配任何高中科目且非明显无关，请返回"无法确定"

输出要求：必须只返回标准 JSON 格式，不附加任何解释。格式如下：
如果类别属于六个科目：{"category": "物理", "keywords": ["牛顿第二定律", "加速度", "匀变速"]}
如果类别是"无关文件"或"无法确定"：{"category": "无关文件", "keywords": []}

注意：keywords 必须是该科目常用的中国高中术语，最多提取 3~5 个。`

// AIResult AI分析结果
type AIResult struct {
	Category string   `json:"category"`
	Keywords []string `json:"keywords"`
}

// Client AI客户端
type AIClient struct {
	endpoint   string
	apiKey     string
	model      string
	retryWait  int // 秒
	maxRetries int
	httpClient *http.Client
}

// NewClient 创建AI客户端
func NewAIClient(endpoint, apiKey, model string, retryWait, maxRetries int) *AIClient {
	return &AIClient{
		endpoint:   endpoint,
		apiKey:     apiKey,
		model:      model,
		retryWait:  retryWait,
		maxRetries: maxRetries,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// chatRequest OpenAI兼容的请求体
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Analyze 执行AI分析（含重试机制）
// lightMode=true 仅分析文件名；false 分析正文片段
func (c *AIClient) Analyze(content string, lightMode bool) (*AIResult, error) {
	mode := "文件名"
	if !lightMode {
		mode = "正文片段"
	}

	var lastErr error
	attempts := c.maxRetries + 1

	for attempt := 1; attempt <= attempts; attempt++ {
		result, err := c.call(content)
		if err == nil && result != nil {
			return result, nil
		}

		lastErr = err
		if err != nil {
			if attempt < attempts {
				// 不是最后一次，等待重试
				waitMsg := fmt.Sprintf("[WARN] [AI调用失败] %s 失败 (第%d次)，%d秒后重试... 错误: %v", mode, attempt, c.retryWait, err)
				return nil, fmt.Errorf("need_retry:%s", waitMsg)
			}
			return nil, fmt.Errorf("[ERROR] [AI重试失败] %s 已重试%d次仍失败: %v", mode, attempt, err)
		}
		if result == nil {
			return nil, fmt.Errorf("[ERROR] [AI返回无效] %s AI返回空结果", mode)
		}
	}

	// 重试次数用尽
	return nil, fmt.Errorf("[ERROR] [AI调用失败] %s 重试次数用尽: %v", mode, lastErr)
}

func (c *AIClient) call(content string) (*AIResult, error) {
	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: content},
		},
		Temperature: 0.1,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("请求序列化失败: %w", err)
	}

	req, err := http.NewRequest("POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络错误: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	// 检查API返回的错误
	if chatResp.Error != nil && chatResp.Error.Message != "" {
		return nil, fmt.Errorf("API错误: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("AI返回空choices")
	}

	contentStr := chatResp.Choices[0].Message.Content
	return parseResult(contentStr)
}

// parseResult 从AI返回内容中提取JSON结果
func parseResult(content string) (*AIResult, error) {
	// 尝试直接解析
	var result AIResult
	if err := json.Unmarshal([]byte(content), &result); err == nil {
		if result.Category != "" {
			return &result, nil
		}
	}

	// 尝试从文本中提取JSON（处理AI可能输出的额外文字）
	start := -1
	end := -1
	for i := 0; i < len(content); i++ {
		if content[i] == '{' {
			start = i
			break
		}
	}
	if start >= 0 {
		for i := len(content) - 1; i >= start; i-- {
			if content[i] == '}' {
				end = i + 1
				break
			}
		}
	}

	if start >= 0 && end > start {
		jsonStr := content[start:end]
		if err := json.Unmarshal([]byte(jsonStr), &result); err == nil && result.Category != "" {
			return &result, nil
		}
	}

	return nil, fmt.Errorf("无法解析AI返回: %s", content)
}