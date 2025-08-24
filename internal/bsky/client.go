package bsky

import (
	"context"
	"fmt"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
)

type Client struct {
	xrpcClient *xrpc.Client
	handle     string
	did        string
}

func NewClient(handle, password string) (*Client, error) {
	client := &xrpc.Client{
		Host: "https://bsky.social",
	}

	session, err := atproto.ServerCreateSession(context.Background(), client, &atproto.ServerCreateSession_Input{
		Identifier: handle,
		Password:   password,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	client.Auth = &xrpc.AuthInfo{
		AccessJwt:  session.AccessJwt,
		RefreshJwt: session.RefreshJwt,
		Handle:     session.Handle,
		Did:        session.Did,
	}

	return &Client{
		xrpcClient: client,
		handle:     session.Handle,
		did:        session.Did,
	}, nil
}

func (c *Client) GetTimeline(ctx context.Context, limit int64) (*bsky.FeedGetTimeline_Output, error) {
	return bsky.FeedGetTimeline(ctx, c.xrpcClient, "", "", limit)
}

func (c *Client) GetProfile(ctx context.Context, actor string) (*bsky.ActorDefs_ProfileViewDetailed, error) {
	return bsky.ActorGetProfile(ctx, c.xrpcClient, actor)
}

func (c *Client) GetHandle() string {
	return c.handle
}

func (c *Client) GetDID() string {
	return c.did
}

