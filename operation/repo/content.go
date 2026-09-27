// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"encoding/base64"
	"fmt"
)

const repositoryWriteContentBase64Field = "content_base64"

// selectRepositoryWriteContent resolves the public repository-write content
// selector into the base64 representation expected by Forgejo's native
// contents APIs. Selection is based on field presence so explicit empty text
// and explicit zero-byte content remain distinguishable from omission.
func selectRepositoryWriteContent(values map[string]any) (string, error) {
	content, hasContent, err := optionalString(values, "content")
	if err != nil {
		return "", err
	}
	contentBase64, hasContentBase64, err := optionalString(values, repositoryWriteContentBase64Field)
	if err != nil {
		return "", err
	}

	if hasContent == hasContentBase64 {
		return "", fmt.Errorf("exactly one of content or content_base64 must be supplied")
	}
	if hasContent {
		return base64.StdEncoding.EncodeToString([]byte(content)), nil
	}

	if _, err := base64.StdEncoding.Strict().DecodeString(contentBase64); err != nil {
		return "", fmt.Errorf("content_base64 must be valid standard Base64: %w", err)
	}
	return contentBase64, nil
}

// rejectRepositoryWriteContent rejects either public content representation by
// field presence. Delete operations do not carry repository content, including
// explicit empty values.
func rejectRepositoryWriteContent(values map[string]any) error {
	if _, ok := values["content"]; ok {
		return fmt.Errorf("content must be omitted for delete")
	}
	if _, ok := values[repositoryWriteContentBase64Field]; ok {
		return fmt.Errorf("content_base64 must be omitted for delete")
	}
	return nil
}

func optionalString(values map[string]any, name string) (string, bool, error) {
	raw, ok := values[name]
	if !ok {
		return "", false, nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", true, fmt.Errorf("%s must be a string", name)
	}
	return value, true, nil
}
