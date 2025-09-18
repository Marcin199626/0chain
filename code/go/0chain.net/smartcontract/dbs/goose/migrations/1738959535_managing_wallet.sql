-- +goose Up
-- +goose StatementBegin

ALTER TABLE blobbers ADD COLUMN IF NOT EXISTS managing_wallet text default '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE blobbers DROP COLUMN managing_wallet;

-- +goose StatementEnd
