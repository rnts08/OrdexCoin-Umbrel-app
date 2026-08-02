package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Support addresses from the OrdexCoin README, used on the About view for direct
// donations and for the automatic OXC tip feature.
type supportAddress struct {
	Network string `json:"network"`
	Address string `json:"address"`
}

var supportAddresses = []supportAddress{
	{Network: "EVM", Address: "0x6e8e3c2b31424266e7cff59e910df1587c317427"},
	{Network: "BTC", Address: "bc1qzzvcguvqjc6qhwe2y5vy38w2zke7hksukjhm68"},
	{Network: "LTC", Address: "MPfm5QLKH1r9XxgWmH75Gyps4LDfX5c53L"},
	{Network: "SOL", Address: "GGEaCMpnyM8tB5BU4RMuLm6tgMr3q9FgMHodxDxxAGby"},
	{Network: "OXC", Address: "oxc1qcjav0mpjjvufc2zwfddnmep0janwv0czwk657e"},
}

// devOXCAddress is the OrdexCoin (native) donation address used by the auto-tip feature.
const devOXCAddress = "oxc1qcjav0mpjjvufc2zwfddnmep0janwv0czwk657e"

type tipRequest struct {
	Amount      float64 `json:"amount"`
	SubtractFee bool    `json:"subtractFee"`
}

// handleTip sends OXC directly from the wallet to the developer address.
func (s *Server) handleTip(w http.ResponseWriter, r *http.Request) {
	var req tipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Amount <= 0 {
		writeErr(w, http.StatusBadRequest, "tip amount must be greater than zero")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	raw, err := s.rpc.Call(ctx, "sendtoaddress",
		devOXCAddress, req.Amount, "Tip to OrdexNetwork developers", "", req.SubtractFee)
	if err != nil {
		writeRPCErr(w, err)
		return
	}

	var txid string
	if err := json.Unmarshal(raw, &txid); err != nil {
		writeErr(w, http.StatusInternalServerError, "unexpected response from daemon")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"txid":    txid,
		"amount":  req.Amount,
		"address": devOXCAddress,
	})
}
