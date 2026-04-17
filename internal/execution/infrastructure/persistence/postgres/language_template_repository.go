package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"code-execution/internal/execution/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LanguageTemplateRepository struct {
	pool *pgxpool.Pool
}

func NewLanguageTemplateRepository(pool *pgxpool.Pool) *LanguageTemplateRepository {
	return &LanguageTemplateRepository{pool: pool}
}

func (r *LanguageTemplateRepository) FindActiveByLanguage(ctx context.Context, language domain.Language) (*domain.LanguageTemplate, error) {
	query := `SELECT id, language, version, entrypoint, runner_template, compile_command, run_command, enabled, updated_at
	FROM language_templates
	WHERE language = $1 AND enabled = true`

	row := r.pool.QueryRow(ctx, query, string(language))
	template, err := scanLanguageTemplate(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return template, nil
}

func (r *LanguageTemplateRepository) FindByLanguage(ctx context.Context, language domain.Language) (*domain.LanguageTemplate, error) {
	query := `SELECT id, language, version, entrypoint, runner_template, compile_command, run_command, enabled, updated_at
	FROM language_templates
	WHERE language = $1`

	row := r.pool.QueryRow(ctx, query, string(language))
	template, err := scanLanguageTemplate(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return template, nil
}

func (r *LanguageTemplateRepository) Upsert(ctx context.Context, template domain.LanguageTemplate) error {
	if template.ID == "" {
		template.ID = newRandomID()
	}

	if template.UpdatedAt.IsZero() {
		template.UpdatedAt = time.Now().UTC()
	}

	query := `INSERT INTO language_templates (
		id, language, version, entrypoint, runner_template, compile_command, run_command, enabled, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	ON CONFLICT (language)
	DO UPDATE SET
		version = EXCLUDED.version,
		entrypoint = EXCLUDED.entrypoint,
		runner_template = EXCLUDED.runner_template,
		compile_command = EXCLUDED.compile_command,
		run_command = EXCLUDED.run_command,
		enabled = EXCLUDED.enabled,
		updated_at = EXCLUDED.updated_at`

	_, err := r.pool.Exec(
		ctx,
		query,
		template.ID,
		string(template.Language),
		template.Version,
		template.Entrypoint,
		template.RunnerTemplate,
		template.CompileCommand,
		template.RunCommand,
		template.Enabled,
		template.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert language template failed: %w", err)
	}

	return nil
}

func (r *LanguageTemplateRepository) FindEnabled(ctx context.Context) ([]domain.LanguageTemplate, error) {
	query := `SELECT id, language, version, entrypoint, runner_template, compile_command, run_command, enabled, updated_at
	FROM language_templates
	WHERE enabled = true
	ORDER BY language ASC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query enabled language templates failed: %w", err)
	}
	defer rows.Close()

	items := make([]domain.LanguageTemplate, 0)
	for rows.Next() {
		template, scanErr := scanLanguageTemplate(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *template)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled language templates failed: %w", err)
	}

	return items, nil
}

func (r *LanguageTemplateRepository) SeedDefaults(ctx context.Context) error {
	defaults := []domain.LanguageTemplate{
		pythonTemplate(),
		javascriptTemplate(),
		javaTemplate(),
		cppTemplate(),
	}

	for _, template := range defaults {
		existing, err := r.FindByLanguage(ctx, template.Language)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}

		if err := r.Upsert(ctx, template); err != nil {
			return err
		}
	}

	return nil
}

func scanLanguageTemplate(row rowScanner) (*domain.LanguageTemplate, error) {
	var (
		template      domain.LanguageTemplate
		language      string
		compileCmdRaw *string
	)

	err := row.Scan(
		&template.ID,
		&language,
		&template.Version,
		&template.Entrypoint,
		&template.RunnerTemplate,
		&compileCmdRaw,
		&template.RunCommand,
		&template.Enabled,
		&template.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	parsedLanguage, err := domain.ParseLanguage(language)
	if err != nil {
		return nil, err
	}

	template.Language = parsedLanguage
	if compileCmdRaw != nil {
		trimmed := strings.TrimSpace(*compileCmdRaw)
		if trimmed != "" {
			template.CompileCommand = &trimmed
		}
	}

	return &template, nil
}

func pythonTemplate() domain.LanguageTemplate {
	return domain.LanguageTemplate{
		ID:         newRandomID(),
		Language:   domain.LanguagePython,
		Version:    "2026.04",
		Entrypoint: "main.py",
		RunnerTemplate: `# {{USER_CODE}}

def _normalize(value: str) -> str:
	return value.replace("\r\n", "\n").rstrip()

if __name__ == "__main__":
	args = """{{TEST_INPUT}}"""
	expected = """{{EXPECTED_OUTPUT}}"""
	actual = {{FUNCTION_CALL}}
	print(_normalize(str(actual)))
`,
		CompileCommand: nil,
		RunCommand:     "python main.py",
		Enabled:        true,
		UpdatedAt:      time.Now().UTC(),
	}
}

func javascriptTemplate() domain.LanguageTemplate {
	return domain.LanguageTemplate{
		ID:         newRandomID(),
		Language:   domain.LanguageJavaScript,
		Version:    "2026.04",
		Entrypoint: "main.js",
		RunnerTemplate: `// {{USER_CODE}}

function normalize(value) {
	return String(value).replace(/\r\n/g, "\n").trimEnd();
}

const testInput = "{{TEST_INPUT}}";
const expected = "{{EXPECTED_OUTPUT}}";
const actual = {{FUNCTION_CALL}};
process.stdout.write(normalize(actual));
`,
		CompileCommand: nil,
		RunCommand:     "node main.js",
		Enabled:        true,
		UpdatedAt:      time.Now().UTC(),
	}
}

func javaTemplate() domain.LanguageTemplate {
	compile := "javac Main.java"
	return domain.LanguageTemplate{
		ID:         newRandomID(),
		Language:   domain.LanguageJava,
		Version:    "2026.04",
		Entrypoint: "Main.java",
		RunnerTemplate: `import java.util.*;

// {{USER_CODE}}

public class Main {
	static String normalize(String value) {
		return value.replace("\r\n", "\n").replaceAll("\\s+$", "");
	}

	public static void main(String[] args) {
		String input = "{{TEST_INPUT}}";
		String expected = "{{EXPECTED_OUTPUT}}";
		Object actual = {{FUNCTION_CALL}};
		System.out.print(normalize(String.valueOf(actual)));
	}
}
`,
		CompileCommand: &compile,
		RunCommand:     "java Main",
		Enabled:        true,
		UpdatedAt:      time.Now().UTC(),
	}
}

func cppTemplate() domain.LanguageTemplate {
	compile := "g++ -std=c++20 -O2 -o main main.cpp"
	return domain.LanguageTemplate{
		ID:         newRandomID(),
		Language:   domain.LanguageCPP,
		Version:    "2026.04",
		Entrypoint: "main.cpp",
		RunnerTemplate: `#include <bits/stdc++.h>
using namespace std;

// {{USER_CODE}}

template <typename T>
string toPrintable(const T& value) {
	ostringstream out;
	out << value;
	return out.str();
}

string normalize(string value) {
	while (!value.empty() && (value.back() == '\n' || value.back() == '\r' || value.back() == ' ' || value.back() == '\t')) {
		value.pop_back();
	}
	return value;
}

int main() {
	string input = R"({{TEST_INPUT}})";
	string expected = R"({{EXPECTED_OUTPUT}})";
	auto actual = {{FUNCTION_CALL}};
	cout << normalize(toPrintable(actual));
	return 0;
}
`,
		CompileCommand: &compile,
		RunCommand:     "./main",
		Enabled:        true,
		UpdatedAt:      time.Now().UTC(),
	}
}
func newRandomID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}
