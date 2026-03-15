package books

import "app-backend/internal/domain/contract"

type domainPack struct {
	queryBuilder queryBuilder
	matchEngine  matchEngine
	namingEngine namingEngine
	importRules  importRules
}

func NewDomain() contract.Domain {
	return &domainPack{
		queryBuilder: queryBuilder{},
		matchEngine:  matchEngine{},
		namingEngine: namingEngine{},
		importRules:  importRules{},
	}
}

func (d *domainPack) Name() string { return "books" }

func (d *domainPack) QueryBuilder() contract.QueryBuilder { return d.queryBuilder }

func (d *domainPack) MatchEngine() contract.MatchEngine { return d.matchEngine }

func (d *domainPack) NamingEngine() contract.NamingEngine { return d.namingEngine }

func (d *domainPack) ImportRules() contract.ImportRules { return d.importRules }
