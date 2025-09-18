-- +goose Up
-- +goose StatementBegin
ALTER TABLE allocations ADD COLUMN owner_signing_public_key TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE allocations DROP COLUMN owner_signing_public_key;
-- +goose StatementEnd
