package main

import (
	"errors"
	"golearn/mock/mocks"
	"golearn/mock/models"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestUserServiceGetName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockUserRepo(ctrl)
	mockRepo.EXPECT().GetUser(100).Return(&models.User{ID: 100, Name: "Tom"}, nil)

	svc := models.NewUserService(mockRepo)
	name, err := svc.GetName(100)
	assert.NoError(t, err)
	assert.Equal(t, "Tom", name)
}

func TestUserService_GetName_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockUserRepo(ctrl)
	mockRepo.EXPECT().
		GetUser(999).
		Return(nil, errors.New("user not found"))

	svc := models.NewUserService(mockRepo)
	name, err := svc.GetName(999)

	assert.ErrorContains(t, err, "user not found")
	assert.Empty(t, name)
}
