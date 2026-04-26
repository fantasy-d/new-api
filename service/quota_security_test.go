package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecreaseUserQuota_Insufficient(t *testing.T) {
	truncate(t)

	const userID = 101
	seedUser(t, userID, 10)

	err := model.DecreaseUserQuota(userID, 11, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user quota is not enough")
	assert.Equal(t, 10, getUserQuota(t, userID))
}

func TestDecreaseTokenQuota_Insufficient(t *testing.T) {
	truncate(t)

	const userID, tokenID = 102, 201
	seedUser(t, userID, 100)
	seedToken(t, tokenID, userID, "sk-atomic-test", 10)

	err := model.DecreaseTokenQuota(tokenID, "sk-atomic-test", 11)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token quota is not enough")
	assert.Equal(t, 10, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, 0, getTokenUsedQuota(t, tokenID))
}

func TestCalculateAudioQuota_RoundsUpPositiveFraction(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	assert.Equal(t, 1, calculateAudioQuota(QuotaInfo{
		UsePrice:      true,
		ModelPrice:    0.4,
		GroupRatio:    1,
		ModelName:     "test-model",
		ModelRatio:    1,
		InputDetails:  TokenDetails{},
		OutputDetails: TokenDetails{},
	}))

	assert.Equal(t, 1, calculateAudioQuota(QuotaInfo{
		UsePrice:   false,
		ModelName:  "test-model",
		ModelRatio: 0.4,
		GroupRatio: 1,
		InputDetails: TokenDetails{
			TextTokens: 1,
		},
		OutputDetails: TokenDetails{},
	}))
}

func newRealtimeTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	return ctx
}

func newRealtimeRelayInfo(userID, tokenID int, tokenKey string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		UserId:          userID,
		TokenId:         tokenID,
		TokenKey:        tokenKey,
		TokenUnlimited:  false,
		IsPlayground:    true,
		UsingGroup:      "default",
		UserGroup:       "default",
		OriginModelName: "test-realtime-model",
		StartTime:       time.Now(),
		UserSetting:     dto.UserSetting{},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId: 1,
		},
		PriceData: types.PriceData{
			UsePrice:   true,
			ModelPrice: 1,
			GroupRatioInfo: types.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
	}
}

func TestWssBilling_PreConsumeAndSettle(t *testing.T) {
	truncate(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	const userID, tokenID = 103, 202
	seedUser(t, userID, 5)
	seedChannel(t, 1)

	ctx := newRealtimeTestContext()
	ctx.Set("token_quota", 5)
	info := newRealtimeRelayInfo(userID, tokenID, "wss-test-key")
	usage := &dto.RealtimeUsage{
		TotalTokens: 1,
		InputTokenDetails: dto.InputTokenDetails{
			TextTokens: 1,
		},
		InputTokens: 1,
	}

	require.NoError(t, PreWssConsumeQuota(ctx, info, usage))
	require.NotNil(t, info.Billing)
	assert.Equal(t, 4, getUserQuota(t, userID))

	require.NoError(t, PostWssConsumeQuota(ctx, info, info.OriginModelName, usage, ""))
	assert.Nil(t, info.Billing)
	assert.Equal(t, 4, getUserQuota(t, userID))
}

func TestWssBilling_InsufficientQuota(t *testing.T) {
	truncate(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	const userID, tokenID = 104, 203
	seedUser(t, userID, 0)
	seedChannel(t, 1)

	ctx := newRealtimeTestContext()
	ctx.Set("token_quota", 5)
	info := newRealtimeRelayInfo(userID, tokenID, "wss-no-quota")

	err := PreWssConsumeQuota(ctx, info, &dto.RealtimeUsage{
		TotalTokens: 1,
		InputTokenDetails: dto.InputTokenDetails{
			TextTokens: 1,
		},
		InputTokens: 1,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "用户额度不足")
	assert.Nil(t, info.Billing)
	assert.Equal(t, 0, getUserQuota(t, userID))
}

func TestWssBilling_ZeroQuotaSkipsSession(t *testing.T) {
	truncate(t)

	ctx := newRealtimeTestContext()
	info := newRealtimeRelayInfo(1, 1, "unused")
	info.PriceData.ModelPrice = 0

	require.NoError(t, PreWssConsumeQuota(ctx, info, &dto.RealtimeUsage{}))
	assert.Nil(t, info.Billing)
}
