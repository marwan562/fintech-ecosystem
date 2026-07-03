package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/sapliy/sapliy-core/internal/wallet/domain"
	"github.com/sapliy/sapliy-core/pkg/apierror"
	"github.com/sapliy/sapliy-core/pkg/authutil"
	"github.com/sapliy/sapliy-core/pkg/jsonutil"
	pb "github.com/sapliy/sapliy-core/proto/wallet"
)

type WalletHandler struct {
	service *domain.WalletService
}

func NewWalletHandler(service *domain.WalletService) *WalletHandler {
	return &WalletHandler{service: service}
}

// HTTP Handlers

type TopUpRequest struct {
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	ReferenceId string `json:"reference_id"`
}

type TransferRequest struct {
	ToUserId    string `json:"to_user_id"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	ReferenceId string `json:"reference_id"`
}

func (h *WalletHandler) GetWallet(w http.ResponseWriter, r *http.Request) {
	targetUserID := jsonutil.GetIDFromPath(r, "/v1/wallets/")
	if targetUserID == "" {
		apierror.BadRequest("Missing User ID").Write(w)
		return
	}

	// Verify requester identity
	userID, err := authutil.ExtractUserID(r)
	if err != nil || userID == "" {
		apierror.Unauthorized("Authentication required").Write(w)
		return
	}

	// Optional: Check if userID matches targetUserID or user has admin scope
	// For now, we trust the gateway's scoping logic.

	wallet, err := h.service.GetWallet(r.Context(), targetUserID)
	if err != nil {
		apierror.Internal("Failed to retrieve wallet").Write(w)
		return
	}
	if wallet == nil {
		apierror.NotFound("Wallet not found").Write(w)
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, wallet)
}

func (h *WalletHandler) TopUp(w http.ResponseWriter, r *http.Request) {
	userID, err := authutil.ExtractUserID(r)
	if err != nil || userID == "" {
		apierror.Unauthorized("Authentication required").Write(w)
		return
	}

	var req TopUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	res, err := h.service.TopUp(r.Context(), &pb.TopUpRequest{
		UserId:      userID,
		Amount:      req.Amount,
		Currency:    req.Currency,
		ReferenceId: req.ReferenceId,
	})
	if err != nil {
		apierror.Internal(err.Error()).Write(w)
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, res)
}

func (h *WalletHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	fromUserID, err := authutil.ExtractUserID(r)
	if err != nil || fromUserID == "" {
		apierror.Unauthorized("Authentication required").Write(w)
		return
	}

	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	res, err := h.service.Transfer(r.Context(), &pb.TransferRequest{
		FromUserId:  fromUserID,
		ToUserId:    req.ToUserId,
		Amount:      req.Amount,
		Currency:    req.Currency,
		ReferenceId: req.ReferenceId,
	})
	if err != nil {
		apierror.Internal(err.Error()).Write(w)
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, res)
}

// gRPC Server

type WalletGRPCServer struct {
	pb.UnimplementedWalletServiceServer
	service *domain.WalletService
}

func NewWalletGRPCServer(service *domain.WalletService) *WalletGRPCServer {
	return &WalletGRPCServer{service: service}
}

func (s *WalletGRPCServer) GetWallet(ctx context.Context, req *pb.GetWalletRequest) (*pb.Wallet, error) {
	return s.service.GetWallet(ctx, req.UserId)
}

func (s *WalletGRPCServer) TopUp(ctx context.Context, req *pb.TopUpRequest) (*pb.TransactionResponse, error) {
	return s.service.TopUp(ctx, req)
}

func (s *WalletGRPCServer) Transfer(ctx context.Context, req *pb.TransferRequest) (*pb.TransactionResponse, error) {
	return s.service.Transfer(ctx, req)
}
