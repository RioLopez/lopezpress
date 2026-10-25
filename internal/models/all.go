package models

// All returns every persisted entity, and it is the ONLY such list.
//
// Why it exists: schema bootstrap used to keep two hand-written lists -- one
// for AutoMigrate (11 models) and one for the drift check (9 models). The
// drift list had silently lost PostRevision and SchemaMigration, so the rule
// "a new column needs a hand-written migration" simply did not apply to
// post_revisions. Two lists of the same thing always diverge; one cannot.
// Add a new entity here and both paths pick it up.
func All() []interface{} {
	return []interface{}{
		&User{}, &Post{}, &PostRevision{}, &Category{}, &Tag{}, &PostTag{},
		&PostView{}, &SiteConfig{}, &LoginLog{}, &ActionLog{}, &SchemaMigration{},
		&UploadFile{}, &UploadRef{}, &EmailCode{},
	}
}
