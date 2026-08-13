package agentic

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"deepseek-api/pkg/client"
)

var base64DataURLRegex = regexp.MustCompile(`data:image/([a-zA-Z]+);base64,([A-Za-z0-9+/=]+)`)

func ProcessImagePayload(ctx context.Context, cli *client.Client, rawData string) ([]string, string, error) {
	matches := base64DataURLRegex.FindAllStringSubmatch(rawData, -1)
	if len(matches) == 0 {
		return nil, rawData, nil
	}

	var refFileIDs []string
	cleanText := base64DataURLRegex.ReplaceAllString(rawData, "[IMAGE_ATTACHED]")

	for i, m := range matches {
		if len(m) < 3 {
			continue
		}
		ext := m[1]
		b64Str := m[2]

		imgBytes, err := base64.StdEncoding.DecodeString(b64Str)
		if err != nil {
			return nil, rawData, fmt.Errorf("failed to decode base64 image data: %w", err)
		}

		fileName := fmt.Sprintf("upload_image_%d.%s", i+1, ext)
		fileID, err := cli.UploadFile(ctx, fileName, imgBytes)
		if err != nil {
			return nil, rawData, fmt.Errorf("failed to upload image to DeepSeek: %w", err)
		}
		refFileIDs = append(refFileIDs, fileID)
	}

	return refFileIDs, strings.TrimSpace(cleanText), nil
}
