#!/usr/bin/env sh
set -eu

SQL_DIR="/docker-entrypoint-initdb.d/sql"

if [ ! -d "$SQL_DIR" ]; then
  echo "No SQL directory mounted at $SQL_DIR; skipping AetherLink migrations."
  exit 0
fi

echo "Running AetherLink SQL migrations in numeric order..."

latest_version="$(
  find "$SQL_DIR" -maxdepth 1 -type f -name "*.sql" \
    | awk '
        {
          name = $0
          sub(/^.*\//, "", name)
          sub(/\.sql$/, "", name)
          if (name ~ /^[0-9]+$/ && name + 0 > max) {
            max = name + 0
          }
        }
        END { if (max > 0) print max }
      '
)"

# Feed every migration and the version marker to one psql process. With
# --single-transaction, a failure rolls back the complete first-install chain
# instead of leaving a partially initialized database volume behind.
{
  find "$SQL_DIR" -maxdepth 1 -type f -name "*.sql" \
    | awk '
        {
          path = $0
          name = path
          sub(/^.*\//, "", name)
          version = name
          sub(/\.sql$/, "", version)
          if (version ~ /^[0-9]+$/) {
            printf "%09d %s\n", version, path
          }
        }
      ' \
    | sort -n \
    | sed 's/^[0-9][0-9]* //' \
    | while IFS= read -r sql_file; do
        echo "-- Applying $(basename "$sql_file")"
        cat "$sql_file"
        printf '\n'
      done

  if [ -n "$latest_version" ]; then
    cat <<SQL
CREATE TABLE IF NOT EXISTS public.sys_version (
  version_number int NOT NULL,
  version varchar(255) NOT NULL,
  CONSTRAINT sys_version_pkey PRIMARY KEY (version_number)
);
DELETE FROM public.sys_version;
INSERT INTO public.sys_version (version_number, version)
VALUES ($latest_version, 'bootstrap-' || CAST($latest_version AS text));
SQL
  fi
} | psql --single-transaction -v ON_ERROR_STOP=1 \
  --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -f -

echo "AetherLink SQL migrations completed."
