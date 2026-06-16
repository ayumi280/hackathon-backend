package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	vision "cloud.google.com/go/vision/v2/apiv1"
	visionpb "cloud.google.com/go/vision/v2/apiv1/visionpb"
	"github.com/labstack/echo/v4"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// AIAssistResponse はAIアシストのレスポンス型
type AIAssistResponse struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
	SuggestPrice int      `json:"suggest_price"`
	VisionLabels []string `json:"vision_labels"` // Cloud Visionが検出したラベル（デバッグ・UI表示用）
}

// AIAssist は以下の2段階でAIアシストを行う:
//  1. Cloud Vision API → 画像からラベルを検出（何の商品か把握）
//  2. OpenAI GPT-4o  → ラベル＋画像をもとに出品情報をJSON生成
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

	// --- Step 1: Cloud Vision API でラベル検出 ---
	labels, err := detectLabels(ctx, imgBytes)
	if err != nil {
		// Vision APIが失敗しても処理を継続（ラベルなしでGPT-4oへ）
		labels = []string{}
	}

	// --- Step 2: GPT-4oで出品情報を生成 ---
	result, err := generateItemInfo(ctx, imgBytes, file.Header.Get("Content-Type"), labels)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "AI生成に失敗しました: "+err.Error())
	}

	result.VisionLabels = labels
	return c.JSON(http.StatusOK, result)
}

// detectLabels はCloud Vision APIを呼び出し、画像のラベルを検出する
func detectLabels(ctx context.Context, imgBytes []byte) ([]string, error) {
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("Cloud Visionクライアント初期化失敗: %w", err)
	}
	defer client.Close()

	// v2 APIはBatchAnnotateImagesを使う（単画像でも同様）
	batchResp, err := client.BatchAnnotateImages(ctx, &visionpb.BatchAnnotateImagesRequest{
		Requests: []*visionpb.AnnotateImageRequest{
			{
				Image: &visionpb.Image{Content: imgBytes},
				Features: []*visionpb.Feature{
					{
						Type:       visionpb.Feature_LABEL_DETECTION,
						MaxResults: 10, // 上位10件のラベルを取得
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

	// スコアが0.7以上のラベルのみ採用（精度の高いものを絞り込む）
	var labels []string
	for _, annotation := range resp.LabelAnnotations {
		if annotation.Score >= 0.7 {
			labels = append(labels, annotation.Description)
		}
	}
	return labels, nil
}

// generateItemInfo はGPT-4oに画像＋Cloud Visionラベルを渡し、出品情報をJSON生成する
func generateItemInfo(ctx context.Context, imgBytes []byte, mimeType string, labels []string) (*AIAssistResponse, error) {
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = "image/jpeg"
	}

	// base64 data URLに変換（GPT-4oの画像入力形式）
	b64 := base64.StdEncoding.EncodeToString(imgBytes)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	// Cloud Visionのラベルをプロンプトに組み込む
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

	client := openai.NewClient(
		option.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	)

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4o,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
				// 画像（base64 data URL）
				openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL:    dataURL,
					Detail: "low", // コスト削減: low解像度で十分
				}),
				// テキストプロンプト
				openai.TextContentPart(prompt),
			}),
		},
		MaxTokens: openai.Int(1024),
	})
	if err != nil {
		return nil, fmt.Errorf("GPT-4o呼び出し失敗: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("GPT-4oからのレスポンスが空です")
	}

	responseText := resp.Choices[0].Message.Content

	var result AIAssistResponse
	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		// JSONパース失敗時のフォールバック（rawテキストを返す）
		return &AIAssistResponse{
			Title:       "解析失敗",
			Description: responseText,
		}, nil
	}

	return &result, nil
}
