package services

import (
	"fmt"
	"net/http"

	ldap "github.com/go-ldap/ldap/v3"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/pkg/config"
	apperrors "request-system/pkg/errors"
)

type ADServiceInterface interface {
	SearchUsers(searchQuery string) ([]dto.ADUserDTO, error)
}

type ADService struct {
	ldapCfg *config.LDAPConfig
	logger  *zap.Logger
}

func NewADService(ldapCfg *config.LDAPConfig, logger *zap.Logger) ADServiceInterface {
	return &ADService{ldapCfg: ldapCfg, logger: logger}
}

func (s *ADService) SearchUsers(searchQuery string) ([]dto.ADUserDTO, error) {
	// Проверка "включателя" из конфига
	if !s.ldapCfg.SearchEnabled {
		s.logger.Warn("Попытка поиска в AD, когда функция отключена")
		return nil, apperrors.NewHttpError(http.StatusServiceUnavailable, "Функция поиска в Active Directory отключена в конфигурации.", nil, nil)
	}

	s.logger.Info("[AD_SEARCH] Начало поиска", zap.String("query", searchQuery))

	l, err := ldap.DialURL(fmt.Sprintf("ldap://%s:%d", s.ldapCfg.Host, s.ldapCfg.Port))
	if err != nil {
		s.logger.Error("[AD_SEARCH] Не удалось подключиться к LDAP-серверу", zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}
	defer l.Close()

	// Аутентификация под сервисной учетной записью
	err = l.Bind(s.ldapCfg.BindDN, s.ldapCfg.BindPassword)
	if err != nil {
		s.logger.Error("[AD_SEARCH] Не удалось выполнить Bind под сервисной учетной записью", zap.Error(err), zap.String("bind_dn", s.ldapCfg.BindDN))
		return nil, apperrors.ErrInternalServer
	}

	// Собираем фильтр из шаблона в конфиге
	escapedQuery := ldap.EscapeFilter(searchQuery)

	// !!! ВОТ ГЛАВНОЕ ИСПРАВЛЕНИЕ !!!
	// Твой шаблон фильтра ожидает ДВА '%s', поэтому мы передаем ДВА аргумента.
	filter := fmt.Sprintf(s.ldapCfg.SearchFilterPattern, escapedQuery, escapedQuery)

	s.logger.Debug("[AD_SEARCH] Сформирован LDAP фильтр", zap.String("filter", filter))

	searchRequest := ldap.NewSearchRequest(
		s.ldapCfg.SearchBaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		s.ldapCfg.SearchAttributes,
		nil,
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		s.logger.Error("[AD_SEARCH] Ошибка при выполнении поиска в AD", zap.Error(err), zap.String("filter", filter))
		return nil, apperrors.ErrInternalServer
	}

	s.logger.Info("[AD_SEARCH] Поиск завершен успешно", zap.Int("found_users", len(sr.Entries)))

	var users []dto.ADUserDTO
	for _, entry := range sr.Entries {
		users = append(users, dto.ADUserDTO{
			Username: entry.GetAttributeValue(s.ldapCfg.UsernameAttribute),
			FIO:      entry.GetAttributeValue(s.ldapCfg.FIOAttribute),
		})
	}

	return users, nil
}
