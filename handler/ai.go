package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	vision "cloud.google.com/go/vision/v2/apiv1"
	visionpb "cloud.google.com/go/vision/v2/apiv1/visionpb"
	"github.com/google/generative-ai-go/genai"
	"github.com/labstack/echo/v4"
	"google.golang.org/api/option"

	"hackathon-backend/db"
	"hackathon-backend/model"
)

// AIAssistResponse はAIアシストのレスポンス型
type AIAssistResponse struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
	SuggestPrice int      `json:"suggest_price"`
	VisionLabels []string `json:"vision_labels"`
}

// AIAssist は以下の2段階でAIアシストを行う:
//  1. Cloud Vision API → 画像からラベルを検出
//  2. Gemini          → ラベル＋画像をもとに出品情報をJSON生成
func AIAssist(c echo.Context) error {
	file, err := c.FormFile("image")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "画像ファイルが必要です")
	}

	src, err := file.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "ファイルオープン失敗")
	}
	defer src.Close()

	imgBytes, err := io.ReadAll(src)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "ファイル読み込み失敗")
	}

	ctx := context.Background()

	labels, err := detectLabels(ctx, imgBytes)
	if err != nil {
		labels = []string{}
	}

	result, err := generateItemInfo(ctx, imgBytes, file.Header.Get("Content-Type"), labels)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "AI生成に失敗しました: "+err.Error())
	}

	result.VisionLabels = labels
	return c.JSON(http.StatusOK, result)
}

// AIQnA は商品に関する質問に Gemini が回答する
func AIQnA(c echo.Context) error {
	var req struct {
		ItemID   uint   `json:"item_id"`
		Question string `json:"question"`
	}
	if err := c.Bind(&req); err != nil || req.Question == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "item_id と question は必須です")
	}

	var item model.Item
	if result := db.DB.Preload("Tags").First(&item, req.ItemID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "商品が見つかりません")
	}

	ctx := context.Background()
	answer, err := answerQuestion(ctx, item, req.Question)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "回答生成に失敗しました: "+err.Error())
	}

	return c.JSON(http.StatusOK, map[string]string{"answer": answer})
}

// detectLabels はCloud Vision APIを呼び出し、画像のラベルを検出する
func detectLabels(ctx context.Context, imgBytes []byte) ([]string, error) {
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("Cloud Visionクライアント初期化失敗: %w", err)
	}
	defer client.Close()

	batchResp, err := client.BatchAnnotateImages(ctx, &visionpb.BatchAnnotateImagesRequest{
		Requests: []*visionpb.AnnotateImageRequest{
			{
				Image: &visionpb.Image{Content: imgBytes},
				Features: []*visionpb.Feature{
					{
						Type:       visionpb.Feature_LABEL_DETECTION,
						MaxResults: 10,
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ラベル検出失敗: %w", err)
	}
	if len(batchResp.Responses) == 0 {
		return nil, fmt.Errorf("Cloud Visionのレスポンスが空です")
	}

	resp := batchResp.Responses[0]
	if resp.Error != nil {
		return nil, fmt.Errorf("Cloud Vision APIエラー: %s", resp.Error.Message)
	}

	var labels []string
	for _, annotation := range resp.LabelAnnotations {
		if annotation.Score >= 0.7 {
			labels = append(labels, annotation.Description)
		}
	}
	return labels, nil
}

// newGeminiClient は GEMINI_API_KEY を使って Gemini クライアントを生成する
func newGeminiClient(ctx context.Context) (*genai.Client, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY が未設定です")
	}
	return genai.NewClient(ctx, option.WithAPIKey(apiKey))
}

// generateItemInfo は Gemini に画像＋Cloud Vision ラベルを渡し、出品情報をJSON生成する
func generateItemInfo(ctx context.Context, imgBytes []byte, mimeType string, labels []string) (*AIAssistResponse, error) {
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = "image/jpeg"
	}

	client, err := newGeminiClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	labelHint := ""
	if len(labels) > 0 {
		labelHint = fmt.Sprintf("\n\n【Cloud Vision APIが検出したラベル（参考情報）】\n%s",
			strings.Join(labels, ", "))
	}

	prompt := fmt.Sprintf(`この商品画像を分析して、日本のフリマアプリ向けの出品情報をJSON形式で生成してください。%s

必ず以下のJSON形式のみで返答してください（コードブロックなし・説明文なし）：
{
  "title": "商品タイトル（30文字以内、簡潔に）",
  "description": "商品説明文（100〜200文字。状態・特徴・素材・サイズなどを含める）",
  "category": "カテゴリ名（ファッション/家電/本・漫画/スポーツ/インテリア/コスメ/おもちゃ/その他 から1つ）",
  "tags": ["タグ1", "タグ2", "タグ3"],
  "suggest_price": 日本円の相場価格（整数、円記号なし）
}`, labelHint)

	m := client.GenerativeModel("gemini-2.5-flash")
	// genai.ImageData は内部で "image/" を付加するため、サブタイプのみ渡す
	mimeSubtype := strings.TrimPrefix(mimeType, "image/")
	resp, err := m.GenerateContent(ctx,
		genai.ImageData(mimeSubtype, imgBytes),
		genai.Text(prompt),
	)
	if err != nil {
		return nil, fmt.Errorf("Gemini呼び出し失敗: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("Geminiからのレスポンスが空です")
	}

	responseText := extractText(resp.Candidates[0].Content.Parts)
	responseText = extractJSON(responseText)

	var result AIAssistResponse
	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		return &AIAssistResponse{
			Title:       "解析失敗",
			Description: responseText,
		}, nil
	}
	return &result, nil
}

// answerQuestion は商品情報をコンテキストとして Gemini に質問回答させる
func answerQuestion(ctx context.Context, item model.Item, question string) (string, error) {
	client, err := newGeminiClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	tagNames := make([]string, len(item.Tags))
	for i, t := range item.Tags {
		tagNames[i] = t.Name
	}

	prompt := fmt.Sprintf(`あなたはフリマアプリのAIアシスタントです。
以下の商品情報をもとに、購入者からの質問に日本語で丁寧に回答してください。
回答は2〜4文で簡潔にまとめてください。

【商品情報】
タイトル: %s
説明: %s
価格: %d円
タグ: %s

【質問】
%s`, item.Title, item.Description, item.Price, strings.Join(tagNames, ", "), question)

	m := client.GenerativeModel("gemini-2.5-flash")
	resp, err := m.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("Gemini呼び出し失敗: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("Geminiからのレスポンスが空です")
	}

	return extractText(resp.Candidates[0].Content.Parts), nil
}

// extractText は Gemini レスポンスの Parts からテキストを結合して返す
func extractText(parts []genai.Part) string {
	var sb strings.Builder
	for _, part := range parts {
		if t, ok := part.(genai.Text); ok {
			sb.WriteString(string(t))
		}
	}
	return strings.TrimSpace(sb.String())
}

// extractJSON はテキストからJSON部分を抽出する（コードブロック混入対策）
func extractJSON(text string) string {
	if idx := strings.Index(text, "```json"); idx != -1 {
		text = text[idx+7:]
		if end := strings.Index(text, "```"); end != -1 {
			text = text[:end]
		}
	} else if idx := strings.Index(text, "```"); idx != -1 {
		text = text[idx+3:]
		if end := strings.Index(text, "```"); end != -1 {
			text = text[:end]
		}
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end != -1 && end > start {
		return text[start : end+1]
	}
	return strings.TrimSpace(text)
}
