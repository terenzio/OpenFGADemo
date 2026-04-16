package fga

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/client"
	"github.com/openfga/language/pkg/go/transformer"
)

// CreateStore creates a new OpenFGA store and returns its ID.
func CreateStore(ctx context.Context, apiURL, storeName string) (string, error) {
	cfg := &client.ClientConfiguration{
		ApiUrl: apiURL,
	}
	sdk, err := client.NewSdkClient(cfg)
	if err != nil {
		return "", fmt.Errorf("creating OpenFGA client for store creation: %w", err)
	}

	resp, err := sdk.CreateStore(ctx).Body(client.ClientCreateStoreRequest{
		Name: storeName,
	}).Execute()
	if err != nil {
		return "", fmt.Errorf("creating store %q: %w", storeName, err)
	}
	return resp.GetId(), nil
}

// Bootstrap reads an FGA DSL model file, transforms it to JSON, and writes
// the authorization model to the OpenFGA store. It returns the model ID.
func (c *Client) Bootstrap(ctx context.Context, modelPath string) (string, error) {
	dsl, err := os.ReadFile(modelPath)
	if err != nil {
		return "", fmt.Errorf("reading model file %q: %w", modelPath, err)
	}

	jsonStr, err := transformer.TransformDSLToJSON(string(dsl))
	if err != nil {
		return "", fmt.Errorf("transforming DSL to JSON: %w", err)
	}

	var modelReq openfga.WriteAuthorizationModelRequest
	if err := json.Unmarshal([]byte(jsonStr), &modelReq); err != nil {
		return "", fmt.Errorf("unmarshalling model JSON: %w", err)
	}

	resp, err := c.sdk.WriteAuthorizationModel(ctx).Body(modelReq).Execute()
	if err != nil {
		return "", fmt.Errorf("writing authorization model: %w", err)
	}
	return resp.GetAuthorizationModelId(), nil
}

// EnsureStoreAndModel is a one-call setup that creates a store, creates a
// client for it, and writes the authorization model. It returns the configured
// client and the model ID.
func EnsureStoreAndModel(ctx context.Context, apiURL, storeName, modelPath string) (*Client, string, error) {
	storeID, err := CreateStore(ctx, apiURL, storeName)
	if err != nil {
		return nil, "", err
	}

	c, err := NewClient(apiURL, storeID)
	if err != nil {
		return nil, "", err
	}

	modelID, err := c.Bootstrap(ctx, modelPath)
	if err != nil {
		return nil, "", err
	}

	return c, modelID, nil
}
