package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vertix-network/vertix/x/rwa/types"
)

func TestMsgUpdateParams(t *testing.T) {
	k, srv, ctx := setupMsgServer(t)

	params := types.DefaultParams()
	testCases := []struct {
		name   string
		input  *types.MsgUpdateParams
		expErr bool
	}{
		{
			name: "invalid authority",
			input: &types.MsgUpdateParams{
				Authority: sdkAcc(t).String(),
				Params:    params,
			},
			expErr: true,
		},
		{
			name: "invalid params",
			input: &types.MsgUpdateParams{
				Authority: k.GetAuthority(),
				Params:    types.RWAParams{},
			},
			expErr: true,
		},
		{
			name: "all good",
			input: &types.MsgUpdateParams{
				Authority: k.GetAuthority(),
				Params:    params,
			},
			expErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := srv.UpdateParams(ctx, tc.input)
			if tc.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
