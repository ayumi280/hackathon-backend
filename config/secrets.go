package config

import (
	"context"
	"fmt"
	"log"
	"os"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/joho/godotenv"
)

// secretMapping はSecret Manager上のシークレット名→環境変数名のマッピング
var secretMapping = map[string]string{
	"jwt-secret":      "JWT_SECRET",
	"gemini-api-key":  "GEMINI_API_KEY",
	"mysql-user":      "MYSQL_USER",
	"mysql-pwd":       "MYSQL_PWD",
	"mysql-host":      "MYSQL_HOST",
	"mysql-database":  "MYSQL_DATABASE",
	"gcs-bucket":      "GCS_BUCKET",
}

// Load は実行環境に応じてシークレットを読み込む。
// Cloud Run上（K_SERVICE環境変数あり）はSecret Managerから、
// ローカル開発時は.envファイルからフォールバックする。
func Load(ctx context.Context) {
	if os.Getenv("K_SERVICE") == "" {
		// ローカル開発: .envファイルを読み込む
		if err := godotenv.Load(); err != nil {
			log.Println(".envファイルが見つかりません（Cloud Run環境では正常）")
		}
		log.Println("設定ソース: .env ファイル（ローカル開発）")
		return
	}

	// Cloud Run上: Secret Managerから読み込む
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		log.Fatal("GOOGLE_CLOUD_PROJECTが未設定です")
	}

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		log.Fatalf("Secret Managerクライアント初期化失敗: %v", err)
	}
	defer client.Close()

	for secretName, envKey := range secretMapping {
		val, err := accessSecret(ctx, client, projectID, secretName)
		if err != nil {
			// シークレットが未登録の場合はスキップ（既存環境変数を維持）
			log.Printf("シークレット取得スキップ [%s]: %v", secretName, err)
			continue
		}
		os.Setenv(envKey, val)
		log.Printf("Secret Manager → 環境変数設定完了: %s", envKey)
	}

	log.Println("設定ソース: Secret Manager（Cloud Run環境）")
}

// accessSecret は指定したシークレットの最新バージョンを取得する
func accessSecret(ctx context.Context, client *secretmanager.Client, projectID, name string) (string, error) {
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectID, name),
	}
	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", err
	}
	return string(result.Payload.Data), nil
}
