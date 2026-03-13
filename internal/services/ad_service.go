package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	ldap "github.com/go-ldap/ldap/v3"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/pkg/config"
	apperrors "request-system/pkg/errors"
)

type ADServiceInterface interface {
	SearchUsers(searchQuery string) ([]dto.ADUserDTO, error)
	FindExactUsernames(localParts []string) (map[string]string, error)
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

func (s *ADService) FindExactUsernames(localParts []string) (map[string]string, error) {
	if !s.ldapCfg.SearchEnabled {
		s.logger.Warn("attempt to search AD while LDAP search is disabled")
		return nil, apperrors.NewHttpError(
			http.StatusServiceUnavailable,
			"Функция поиска в Active Directory отключена в конфигурации.",
			nil,
			nil,
		)
	}

	unique := make(map[string]string, len(localParts))
	for _, part := range localParts {
		clean := strings.TrimSpace(part)
		if clean == "" {
			continue
		}

		lower := strings.ToLower(clean)
		if _, exists := unique[lower]; !exists {
			unique[lower] = clean
		}
	}

	result := make(map[string]string, len(unique))
	if len(unique) == 0 {
		return result, nil
	}

	conn, err := ldap.DialURL(fmt.Sprintf("ldap://%s:%d", s.ldapCfg.Host, s.ldapCfg.Port))
	if err != nil {
		s.logger.Error("[AD_EXACT] LDAP dial failed", zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}
	defer conn.Close()
	conn.SetTimeout(10 * time.Second)

	if err := conn.Bind(s.ldapCfg.BindDN, s.ldapCfg.BindPassword); err != nil {
		s.logger.Error("[AD_EXACT] LDAP bind failed", zap.String("bind_dn", s.ldapCfg.BindDN), zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}

	placeholders := strings.Count(s.ldapCfg.SearchFilterPattern, "%s")
	buildFilter := func(query string) string {
		escaped := ldap.EscapeFilter(query)
		switch {
		case placeholders <= 0:
			return s.ldapCfg.SearchFilterPattern
		case placeholders == 1:
			return fmt.Sprintf(s.ldapCfg.SearchFilterPattern, escaped)
		default:
			args := make([]interface{}, placeholders)
			for i := range args {
				args[i] = escaped
			}
			return fmt.Sprintf(s.ldapCfg.SearchFilterPattern, args...)
		}
	}

	for lower, localPart := range unique {
		filter := buildFilter(localPart)
		searchRequest := ldap.NewSearchRequest(
			s.ldapCfg.SearchBaseDN,
			ldap.ScopeWholeSubtree,
			ldap.NeverDerefAliases,
			0,
			0,
			false,
			filter,
			s.ldapCfg.SearchAttributes,
			nil,
		)

		sr, searchErr := conn.Search(searchRequest)
		if searchErr != nil {
			s.logger.Warn("[AD_EXACT] LDAP search failed for local-part", zap.String("local_part", localPart), zap.Error(searchErr))
			continue
		}

		for _, entry := range sr.Entries {
			username := strings.TrimSpace(entry.GetAttributeValue(s.ldapCfg.UsernameAttribute))
			if strings.EqualFold(username, localPart) {
				result[lower] = username
				break
			}
		}
	}

	return result, nil
}
