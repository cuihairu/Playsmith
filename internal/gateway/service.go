package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "playsmith/internal/gatewaypb"
	"playsmith/internal/sdk"
)

// Service implements the gRPC Gateway surface using the Fabric SDK client.
type Service struct {
	pb.UnimplementedGatewayServer
	Fabric sdk.Client
}

// AuthToken returns the bearer token from gRPC metadata.
func AuthToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", errors.New("missing metadata")
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return "", errors.New("missing authorization metadata")
	}
	const prefix = "Bearer "
	if len(vals[0]) < len(prefix) || vals[0][:len(prefix)] != prefix {
		return "", errors.New("invalid authorization header")
	}
	return vals[0][len(prefix):], nil
}

func (s *Service) Mint(ctx context.Context, r *pb.MintRequest) (*pb.TxResponse, error) {
	res, err := s.Fabric.Submit("playsmith-assets", "Mint", r.TokenId, r.To, r.Uri, strconv.FormatUint(r.Amount, 10))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.TxResponse{Result: res}, nil
}

func (s *Service) BatchMint(ctx context.Context, r *pb.BatchMintRequest) (*pb.TxResponse, error) {
	res, err := s.Fabric.Submit("playsmith-assets", "BatchMint", join(r.Ids), join(r.Tos), joinUints(r.Amounts), join(r.Uris))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.TxResponse{Result: res}, nil
}

func (s *Service) Transfer(ctx context.Context, r *pb.TransferRequest) (*pb.TxResponse, error) {
	res, err := s.Fabric.Submit("playsmith-assets", "Transfer", r.TokenId, r.From, r.To, strconv.FormatUint(r.Amount, 10))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.TxResponse{Result: res}, nil
}

func (s *Service) BatchTransfer(ctx context.Context, r *pb.BatchTransferRequest) (*pb.TxResponse, error) {
	res, err := s.Fabric.Submit("playsmith-assets", "BatchTransfer", join(r.Ids), r.From, r.To, joinUints(r.Amounts))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.TxResponse{Result: res}, nil
}

func (s *Service) ApproveAll(ctx context.Context, r *pb.ApproveAllRequest) (*pb.TxResponse, error) {
	approved := "false"
	if r.Approved {
		approved = "true"
	}
	res, err := s.Fabric.Submit("playsmith-assets", "SetApprovalForAll", r.Owner, r.Operator, approved)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.TxResponse{Result: res}, nil
}

func (s *Service) ListForSale(ctx context.Context, r *pb.ListForSaleRequest) (*pb.TxResponse, error) {
	res, err := s.Fabric.Submit("playsmith-assets", "ListForSale", r.TokenId, r.Seller, strconv.FormatUint(r.Amount, 10), strconv.FormatUint(r.Price, 10))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.TxResponse{Result: res}, nil
}

func (s *Service) ListForSalePrivate(ctx context.Context, r *pb.ListForSalePrivateRequest) (*pb.TxResponse, error) {
	transient := map[string][]byte{"price": []byte(strconv.FormatUint(r.Price, 10))}
	res, err := s.Fabric.SubmitWithTransient("playsmith-assets", "ListForSalePrivate", transient, r.TokenId, r.Seller, strconv.FormatUint(r.Amount, 10))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.TxResponse{Result: res}, nil
}

func (s *Service) CancelListing(ctx context.Context, r *pb.CancelListingRequest) (*pb.TxResponse, error) {
	res, err := s.Fabric.Submit("playsmith-assets", "CancelListing", r.TokenId, r.Seller, r.ListingId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.TxResponse{Result: res}, nil
}

func (s *Service) Buy(ctx context.Context, r *pb.BuyRequest) (*pb.TxResponse, error) {
	res, err := s.Fabric.Submit("playsmith-assets", "Buy", r.TokenId, r.Seller, r.ListingId, r.Buyer, strconv.FormatUint(r.Amount, 10))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.TxResponse{Result: res}, nil
}

func (s *Service) BalanceOf(ctx context.Context, r *pb.BalanceOfRequest) (*pb.BalanceResponse, error) {
	res, err := s.Fabric.Evaluate("playsmith-assets", "BalanceOf", r.TokenId, r.Owner)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.BalanceResponse{Balance: res}, nil
}

func (s *Service) URI(ctx context.Context, r *pb.URIRequest) (*pb.StringResponse, error) {
	res, err := s.Fabric.Evaluate("playsmith-assets", "URI", r.TokenId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.StringResponse{Value: res}, nil
}

func (s *Service) Exists(ctx context.Context, r *pb.ExistsRequest) (*pb.BoolResponse, error) {
	res, err := s.Fabric.Evaluate("playsmith-assets", "Exists", r.TokenId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.BoolResponse{Value: res == "true"}, nil
}

func (s *Service) TotalSupply(ctx context.Context, r *pb.TotalSupplyRequest) (*pb.StringResponse, error) {
	res, err := s.Fabric.Evaluate("playsmith-assets", "TotalSupply", r.TokenId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.StringResponse{Value: res}, nil
}

func join(v []string) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func joinUints(v []uint64) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}
