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

const adExactSearchBatchSize = 100

func NewADService(ldapCfg *config.LDAPConfig, logger *zap.Logger) ADServiceInterface {
	return &ADService{ldapCfg: ldapCfg, logger: logger}
}

func (s *ADService) ldapTimeout() time.Duration {
	if s.ldapCfg.Timeout > 0 {
		return s.ldapCfg.Timeout
	}

	return 10 * time.Second
}

func (s *ADService) usernameAttribute() string {
	clean := strings.TrimSpace(s.ldapCfg.UsernameAttribute)
	if clean == "" {
		return "sAMAccountName"
	}

	return clean
}

func (s *ADService) searchAttributes(extra ...string) []string {
	attrs := make([]string, 0, len(s.ldapCfg.SearchAttributes)+len(extra))
	seen := make(map[string]struct{}, len(s.ldapCfg.SearchAttributes)+len(extra))

	appendAttr := func(attr string) {
		clean := strings.TrimSpace(attr)
		if clean == "" {
			return
		}

		key := strings.ToLower(clean)
		if _, exists := seen[key]; exists {
			return
		}

		seen[key] = struct{}{}
		attrs = append(attrs, clean)
	}

	for _, attr := range s.ldapCfg.SearchAttributes {
		appendAttr(attr)
	}
	for _, attr := range extra {
		appendAttr(attr)
	}

	return attrs
}

func (s *ADService) dialAndBind(logPrefix string) (*ldap.Conn, error) {
	conn, err := ldap.DialURL(fmt.Sprintf("ldap://%s:%d", s.ldapCfg.Host, s.ldapCfg.Port))
	if err != nil {
		s.logger.Error(logPrefix+" LDAP dial failed", zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}

	conn.SetTimeout(s.ldapTimeout())
	if err := conn.Bind(s.ldapCfg.BindDN, s.ldapCfg.BindPassword); err != nil {
		s.logger.Error(logPrefix+" LDAP bind failed", zap.String("bind_dn", s.ldapCfg.BindDN), zap.Error(err))
		conn.Close()
		return nil, apperrors.ErrInternalServer
	}

	return conn, nil
}

func buildExactUsernamesBatchFilter(usernameAttribute string, localParts []string) string {
	var builder strings.Builder
	builder.WriteString("(&(objectClass=person)(|")

	for _, localPart := range localParts {
		builder.WriteString("(")
		builder.WriteString(usernameAttribute)
		builder.WriteString("=")
		builder.WriteString(ldap.EscapeFilter(localPart))
		builder.WriteString(")")
	}

	builder.WriteString("))")
	return builder.String()
}

func buildSearchFilter(pattern, query string) string {
	placeholders := strings.Count(pattern, "%s")
	escaped := ldap.EscapeFilter(query)

	switch {
	case placeholders <= 0:
		return pattern
	case placeholders == 1:
		return fmt.Sprintf(pattern, escaped)
	default:
		args := make([]interface{}, placeholders)
		for i := range args {
			args[i] = escaped
		}
		return fmt.Sprintf(pattern, args...)
	}
}

func (s *ADService) searchExactUsernamesBatch(conn *ldap.Conn, localParts []string) (map[string]string, error) {
	result := make(map[string]string, len(localParts))
	if len(localParts) == 0 {
		return result, nil
	}

	usernameAttribute := s.usernameAttribute()
	searchRequest := ldap.NewSearchRequest(
		s.ldapCfg.SearchBaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		buildExactUsernamesBatchFilter(usernameAttribute, localParts),
		s.searchAttributes(usernameAttribute),
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	for _, entry := range sr.Entries {
		username := strings.TrimSpace(entry.GetAttributeValue(usernameAttribute))
		if username == "" {
			continue
		}

		result[strings.ToLower(username)] = username
	}

	return result, nil
}

func (s *ADService) SearchUsers(searchQuery string) ([]dto.ADUserDTO, error) {
	// Проверка "включателя" из конфига
	if !s.ldapCfg.SearchEnabled {
		s.logger.Warn("Попытка поиска в AD, когда функция отключена")
		return nil, apperrors.NewHttpError(http.StatusServiceUnavailable, "Функция поиска в Active Directory отключена в конфигурации.", nil, nil)
	}

	s.logger.Info("[AD_SEARCH] Начало поиска", zap.String("query", searchQuery))

	conn, err := s.dialAndBind("[AD_SEARCH]")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Собираем фильтр из шаблона в конфиге
	filter := buildSearchFilter(s.ldapCfg.SearchFilterPattern, searchQuery)

	// The config pattern may contain one or more placeholders; buildSearchFilter normalizes it.

	s.logger.Debug("[AD_SEARCH] Сформирован LDAP фильтр", zap.String("filter", filter))

	searchRequest := ldap.NewSearchRequest(
		s.ldapCfg.SearchBaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter,
		s.searchAttributes(s.usernameAttribute(), s.ldapCfg.FIOAttribute),
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		s.logger.Error("[AD_SEARCH] Ошибка при выполнении поиска в AD", zap.Error(err), zap.String("filter", filter))
		return nil, apperrors.ErrInternalServer
	}

	s.logger.Info("[AD_SEARCH] Поиск завершен успешно", zap.Int("found_users", len(sr.Entries)))

	users := make([]dto.ADUserDTO, 0, len(sr.Entries))
	usernameAttribute := s.usernameAttribute()
	for _, entry := range sr.Entries {
		users = append(users, dto.ADUserDTO{
			Username: entry.GetAttributeValue(usernameAttribute),
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
	orderedLocalParts := make([]string, 0, len(localParts))
	for _, part := range localParts {
		clean := strings.TrimSpace(part)
		if clean == "" {
			continue
		}

		lower := strings.ToLower(clean)
		if _, exists := unique[lower]; !exists {
			unique[lower] = clean
			orderedLocalParts = append(orderedLocalParts, clean)
		}
	}

	result := make(map[string]string, len(unique))
	if len(unique) == 0 {
		return result, nil
	}

	conn, err := s.dialAndBind("[AD_EXACT]")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetTimeout(s.ldapTimeout())

	for start := 0; start < len(orderedLocalParts); start += adExactSearchBatchSize {
		end := start + adExactSearchBatchSize
		if end > len(orderedLocalParts) {
			end = len(orderedLocalParts)
		}

		batch := orderedLocalParts[start:end]
		batchMatches, searchErr := s.searchExactUsernamesBatch(conn, batch)
		if searchErr != nil {
			s.logger.Warn("[AD_EXACT] Batched LDAP search failed, retrying one by one", zap.Int("batch_size", len(batch)), zap.Error(searchErr))
			for _, localPart := range batch {
				singleMatches, singleErr := s.searchExactUsernamesBatch(conn, []string{localPart})
				if singleErr != nil {
					s.logger.Warn("[AD_EXACT] LDAP search failed for local-part", zap.String("local_part", localPart), zap.Error(singleErr))
					continue
				}

				if username, found := singleMatches[strings.ToLower(localPart)]; found {
					result[strings.ToLower(localPart)] = username
				}
			}
			continue
		}

		for lower, username := range batchMatches {
			if _, exists := unique[lower]; exists {
				result[lower] = username
			}
		}
	}

	return result, nil
}
