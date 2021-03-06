package main

import (
	"fmt"

	"github.com/go-ldap/ldap/v3"
)

func ldapCheck(cfg config, email string) bool {
	filter := fmt.Sprintf("(mail=%v)", email)

	mail := ldapRequest(cfg, filter)

	return len(mail) > 0
}

func ldapMail(cfg config, users []string) []string {
	var filter string

	if len(users) == 0 {
		return nil
	}

	for _, item := range users {
		filter = fmt.Sprintf("%v(sAMAccountName=%v)", filter, item)
	}

	mail := ldapRequest(cfg, filter)

	return mail
}

func ldapRequest(cfg config, filter string) []string {
	var mail []string

	conn, err := ldapConnect(cfg)

	if err != nil {
		fmt.Printf("LDAP failed: %s", err)
		return nil
	}

	defer conn.Close()

	filter = fmt.Sprintf("(&(objectClass=user)(objectCategory=person)(!(userAccountControl:1.2.840.113556.1.4.803:=2))(|%v))", filter)

	if mail, err = ldapList(conn, cfg.Endpoints.DC.Base, filter); err != nil {
		fmt.Printf("%v", err)
		return nil
	}

	return mail
}

func ldapConnect(cfg config) (*ldap.Conn, error) {
	addr := fmt.Sprintf("%v:%v", cfg.Endpoints.DC.Host, cfg.Endpoints.DC.Port)
	conn, err := ldap.Dial("tcp", addr)

	if err != nil {
		return nil, fmt.Errorf("connection error: %s", err)
	}

	user := fmt.Sprintf("%v@%v", cfg.Credentials.User, cfg.Endpoints.DC.Domain)

	if err := conn.Bind(user, cfg.Credentials.Password); err != nil {
		return nil, fmt.Errorf("binding error: %s", err)
	}

	return conn, nil
}

func ldapList(conn *ldap.Conn, base string, filter string) ([]string, error) {
	var mail []string

	result, err := conn.Search(ldap.NewSearchRequest(
		base,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		[]string{"mail"},
		nil,
	))

	if err != nil {
		return mail, fmt.Errorf("Failed to search users. %s", err)
	}

	for _, entry := range result.Entries {
		mail = append(mail, entry.GetAttributeValue("mail"))
	}

	return mail, nil
}
