package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/raddigo/raddigo/internal/dto"
	"github.com/raddigo/raddigo/internal/model"
	"github.com/raddigo/raddigo/internal/repository"
	"github.com/raddigo/raddigo/internal/utils"
	"github.com/raddigo/raddigo/internal/validation"
)

// AddressService contains CRUD logic for user addresses.
type AddressService struct {
	repo repository.AddressRepository
	now  func() time.Time
	id   func() string
}

// NewAddressService creates an AddressService.
func NewAddressService(repo repository.AddressRepository) *AddressService {
	return &AddressService{
		repo: repo,
		now:  time.Now,
		id:   func() string { return uuid.NewString() },
	}
}

// Create validates and stores a new address for the user.
func (s *AddressService) Create(ctx context.Context, userID string, in dto.AddressRequest) (dto.AddressResponse, error) {
	if err := s.ensureUser(ctx, userID); err != nil {
		return dto.AddressResponse{}, err
	}
	if err := validation.ValidateAddress(in); err != nil {
		return dto.AddressResponse{}, err
	}

	now := s.now()
	address := model.Address{
		ID:        s.id(),
		Type:      model.AddressTypeUser,
		UserID:    &userID,
		Address1:  trimPtr(&in.Address1),
		Address2:  trimPtr(in.Address2),
		Street:    trimPtr(&in.Street),
		City:      trimPtr(&in.City),
		State:     trimPtr(&in.State),
		Country:   trimPtr(&in.Country),
		Pincode:   trimPtr(&in.Pincode),
		Latitude:  in.Latitude,
		Longitude: in.Longitude,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, &address); err != nil {
		return dto.AddressResponse{}, err
	}
	return toAddressResponse(address), nil
}

// List returns a paginated set of addresses for the user.
func (s *AddressService) List(ctx context.Context, userID string, page, pageSize int) (dto.PageResult[dto.AddressResponse], error) {
	if err := s.ensureUser(ctx, userID); err != nil {
		return dto.PageResult[dto.AddressResponse]{}, err
	}

	page, pageSize = dto.NormalizePageParams(page, pageSize)
	offset := (page - 1) * pageSize

	addresses, total, err := s.repo.ListByUser(ctx, userID, pageSize, offset)
	if err != nil {
		return dto.PageResult[dto.AddressResponse]{}, err
	}

	out := make([]dto.AddressResponse, 0, len(addresses))
	for _, a := range addresses {
		out = append(out, toAddressResponse(a))
	}
	return dto.PageResult[dto.AddressResponse]{
		Data:       out,
		Pagination: dto.NewPagination(page, pageSize, total),
	}, nil
}

// Get returns a single address owned by the user.
func (s *AddressService) Get(ctx context.Context, userID, id string) (dto.AddressResponse, error) {
	address, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return dto.AddressResponse{}, err
	}
	return toAddressResponse(address), nil
}

// Update validates and persists changes to an existing address.
func (s *AddressService) Update(ctx context.Context, userID, id string, in dto.AddressRequest) (dto.AddressResponse, error) {
	if err := validation.ValidateAddress(in); err != nil {
		return dto.AddressResponse{}, err
	}
	address, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return dto.AddressResponse{}, err
	}

	address.Address1 = trimPtr(&in.Address1)
	address.Address2 = trimPtr(in.Address2)
	address.Street = trimPtr(&in.Street)
	address.City = trimPtr(&in.City)
	address.State = trimPtr(&in.State)
	address.Country = trimPtr(&in.Country)
	address.Pincode = trimPtr(&in.Pincode)
	address.Latitude = in.Latitude
	address.Longitude = in.Longitude
	address.UpdatedAt = s.now()

	if err := s.repo.Update(ctx, &address); err != nil {
		return dto.AddressResponse{}, err
	}
	return toAddressResponse(address), nil
}

// Delete removes an address owned by the user.
func (s *AddressService) Delete(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, userID, id)
}

func (s *AddressService) ensureUser(ctx context.Context, userID string) error {
	exists, err := s.repo.UserExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return utils.ErrNotFound
	}
	return nil
}

func toAddressResponse(a model.Address) dto.AddressResponse {
	return dto.AddressResponse{
		ID:        a.ID,
		Type:      string(a.Type),
		Address1:  a.Address1,
		Address2:  a.Address2,
		Street:    a.Street,
		City:      a.City,
		State:     a.State,
		Country:   a.Country,
		Pincode:   a.Pincode,
		Latitude:  a.Latitude,
		Longitude: a.Longitude,
	}
}
