// Package fga provides a wrapper around the OpenFGA Go SDK for permission
// checks, tuple management, and relationship queries.
package fga

import (
	"context"
	"fmt"

	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/client"
)

// TupleKey is an alias for the OpenFGA SDK TupleKey type.
type TupleKey = openfga.TupleKey

// UsersetTree is an alias for the OpenFGA SDK UsersetTree type.
type UsersetTree = openfga.UsersetTree

// Client wraps the OpenFGA Go SDK client with convenience methods.
type Client struct {
	sdk *client.OpenFgaClient
}

// NewClient creates a new OpenFGA client wrapper.
func NewClient(apiURL, storeID string) (*Client, error) {
	cfg := &client.ClientConfiguration{
		ApiUrl:  apiURL,
		StoreId: storeID,
	}
	sdk, err := client.NewSdkClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating OpenFGA client: %w", err)
	}
	return &Client{sdk: sdk}, nil
}

// SDK returns the underlying OpenFGA SDK client for advanced use.
func (c *Client) SDK() *client.OpenFgaClient {
	return c.sdk
}

// Check returns whether the given user has the specified relation on the object.
func (c *Client) Check(ctx context.Context, user, relation, object string) (bool, error) {
	resp, err := c.sdk.Check(ctx).Body(client.ClientCheckRequest{
		User:     user,
		Relation: relation,
		Object:   object,
	}).Execute()
	if err != nil {
		return false, fmt.Errorf("check %s#%s@%s: %w", object, relation, user, err)
	}
	return resp.GetAllowed(), nil
}

// WriteTuples writes relationship tuples to the OpenFGA store.
func (c *Client) WriteTuples(ctx context.Context, tuples []TupleKey) error {
	writes := make([]client.ClientTupleKey, len(tuples))
	for i, t := range tuples {
		writes[i] = client.ClientTupleKey{
			User:     t.User,
			Relation: t.Relation,
			Object:   t.Object,
		}
	}
	_, err := c.sdk.Write(ctx).Body(client.ClientWriteRequest{
		Writes: writes,
	}).Execute()
	if err != nil {
		return fmt.Errorf("write tuples: %w", err)
	}
	return nil
}

// DeleteTuples deletes relationship tuples from the OpenFGA store.
func (c *Client) DeleteTuples(ctx context.Context, tuples []TupleKey) error {
	deletes := make([]client.ClientTupleKeyWithoutCondition, len(tuples))
	for i, t := range tuples {
		deletes[i] = client.ClientTupleKeyWithoutCondition{
			User:     t.User,
			Relation: t.Relation,
			Object:   t.Object,
		}
	}
	_, err := c.sdk.Write(ctx).Body(client.ClientWriteRequest{
		Deletes: deletes,
	}).Execute()
	if err != nil {
		return fmt.Errorf("delete tuples: %w", err)
	}
	return nil
}

// ListObjects returns the objects of a given type that the user has a relation to.
func (c *Client) ListObjects(ctx context.Context, user, relation, objectType string) ([]string, error) {
	resp, err := c.sdk.ListObjects(ctx).Body(client.ClientListObjectsRequest{
		User:     user,
		Relation: relation,
		Type:     objectType,
	}).Execute()
	if err != nil {
		return nil, fmt.Errorf("list objects %s#%s@%s: %w", objectType, relation, user, err)
	}
	return resp.GetObjects(), nil
}

// Expand returns the userset tree for a given relation on an object.
func (c *Client) Expand(ctx context.Context, relation, object string) (*UsersetTree, error) {
	resp, err := c.sdk.Expand(ctx).Body(client.ClientExpandRequest{
		Relation: relation,
		Object:   object,
	}).Execute()
	if err != nil {
		return nil, fmt.Errorf("expand %s#%s: %w", object, relation, err)
	}
	if resp.HasTree() {
		tree := resp.GetTree()
		return &tree, nil
	}
	return nil, nil
}

// Tuple is a convenience constructor for a TupleKey.
func Tuple(user, relation, object string) TupleKey {
	return TupleKey{
		User:     user,
		Relation: relation,
		Object:   object,
	}
}
