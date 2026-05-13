VAS A IMPLEMENTAR

# Arquitectura de Microservicio: Code Execution

Este servicio consume solicitudes de ejecución provenientes de `Challenges & Solutions`, ejecuta código de estudiantes en un entorno aislado y publica resultados para actualizar estado funcional y habilitar análisis posterior.

---

## **1. Módulos de Dominio (Modelos)**

El servicio se organiza alrededor del procesamiento asíncrono de ejecuciones y un catálogo de plantillas por lenguaje para construir scripts de ejecución consistentes.

### **Módulo: Execution Jobs (Aggregate Root)**
Representa el ciclo de vida completo de una ejecución solicitada por evento.

```typescript
// Entidad Principal: ExecutionJob
{
		id: uuid;
		sourceEventId: uuid;                  // eventId recibido desde Challenges (idempotencia)
		solutionId: uuid;
		language: "python" | "javascript" | "java" | "cpp";
		status: "QUEUED" | "STARTED" | "COMPLETED" | "FAILED";

		// Seguridad y límites de runtime
		timeoutMs: number;                    // Límite por test
		memoryLimitMb: number;                // Límite por ejecución
		cpuLimitMs: number;                   // Cuota de CPU por test

		totalExecutionTimeMs: number | null;
		globalError: string | null;           // Error global (compilación, timeout total, runtime fatal)

		createdAt: datetime;
		startedAt: datetime | null;
		completedAt: datetime | null;
}

// Entidad: ExecutionTestResult
{
		id: uuid;
		executionJobId: uuid;
		testId: uuid;
		passed: boolean;
		isHidden: boolean;
		inputHash: string;                    // Hash del input para trazabilidad sin exponer datos sensibles
		actualOutput: string;
		expectedOutput: string;
		errorMessage: string | null;          // Error por test (si aplica)
		executionTimeMs: number;
}
```

### **Módulo: Language Template Registry**
Define cómo se arma el script final para cada lenguaje soportado.

```typescript
// Entidad: LanguageTemplate
{
		id: uuid;
		language: "python" | "javascript" | "java" | "cpp";
		version: string;                      // Ej: "2026.04"
		entrypoint: string;                   // Ej: "main.py", "Main.java", "main.cpp"

		// Plantilla de ejecución para inyectar código y correr casos de prueba
		runnerTemplate: string;               // Debe incluir placeholders: {{USER_CODE}}, {{TEST_INPUT}}, {{EXPECTED_OUTPUT}}

		compileCommand: string | null;        // null para lenguajes interpretados
		runCommand: string;
		enabled: boolean;
		updatedAt: datetime;
}
```

---

## **2. Responsabilidades por Componente (C4)**

En el modelo C4 actual ([index.ts](../index.ts)), Code Execution se divide en cinco componentes:

### **A. Execution Orchestrator**
- **Responsabilidad:** Coordina el pipeline completo de ejecución.
- **Integración C4:** consume `SolutionExecutionRequestedEvent` y publica `SolutionExecutionStartedEvent` y `SolutionExecutionCompletedEvent`.
- **Persistencia:** guarda trazas y métricas en Code Execution DB.

### **B. Code Validator**
- **Responsabilidad:** Detecta instrucciones inseguras o maliciosas antes de ejecutar.
- **Límites:** si falla la validación, el flujo termina con `globalError` controlado.

### **C. Script Builder**
- **Responsabilidad:** Construye el script/binario temporal usando `language`, `entryFunctionName`, `code` y `testCases`.
- **Contrato:** mapea `testCases.input` (argumentos separados por coma) a invocaciones posicionales de la función objetivo.

### **D. Sandbox**
- **Responsabilidad:** Ejecuta de forma aislada con límites de CPU, memoria y tiempo.
- **Aislamiento:** sin acceso de red saliente y filesystem acotado.

### **E. Execution Observer**
- **Responsabilidad:** Observa ejecución, captura output/errores y tiempos por test.
- **Salida:** alimenta al orquestador para persistencia y publicación del evento de completado.

---

## **3. Reglas Críticas de Negocio y Validaciones**

1.  **Consumo Asíncrono Estricto:** El servicio procesa `SolutionExecutionRequestedEvent` desde RabbitMQ. No depende de llamadas síncronas al servicio de Challenges para ejecutar.
2.  **Idempotencia por `eventId`:** Si llega el mismo mensaje más de una vez, no se debe crear una nueva ejecución. Se reutiliza el `ExecutionJob` existente.
3.  **Sandbox Obligatorio:** Toda ejecución ocurre en entorno aislado, sin acceso a red saliente y con límites de CPU/memoria/tiempo para evitar abuso.
4.  **Lenguajes Permitidos por Catálogo:** Solo se ejecutan lenguajes con `LanguageTemplate.enabled=true`. Si no existe template activo, la ejecución finaliza con error controlado.
5.  **Normalización de Output:** Antes de comparar resultados, el servicio normaliza saltos de línea y espacios finales para evitar falsos negativos.
6.  **Manejo de Fallos por Test y Globales:** Errores de compilación o arranque generan `globalError`; errores de un caso puntual se guardan en `ExecutionTestResult.errorMessage`.
7.  **Trazabilidad Completa:** Cada ejecución persiste tiempos, errores y resultados por test para auditoría y para alimentar análisis posterior.
8.  **Separación de Responsabilidades:** Code Execution ejecuta y mide; no asigna puntos ni decide feedback de IA.

---

## **4. Diseño de Endpoints (API REST)**

El flujo principal es event-driven. Estos endpoints son operativos/internos para observabilidad y administración de templates.

| Método | Endpoint | Descripción y Validaciones |
| :--- | :--- | :--- |
| `GET` | `/api/v1/code-execution/health/live` | Liveness del proceso (sin dependencias externas). |
| `GET` | `/api/v1/code-execution/health/ready` | Readiness validando conexión a DB, broker y runtime sandbox. |
| `GET` | `/api/v1/code-execution/languages` | Lista lenguajes habilitados con versión de template y límites efectivos. |
| `GET` | `/api/v1/code-execution/templates/{language}` | Retorna template activo por lenguaje (solo interno/ADMIN). |
| `PUT` | `/api/v1/code-execution/templates/{language}` | Actualiza `runnerTemplate`, comandos y versión. Requiere rol técnico de administración. |
| `GET` | `/api/v1/code-execution/executions/{executionId}` | Consulta estado y resultados persistidos de una ejecución específica. |

---

## **5. Eventos de Integración (RabbitMQ)**

Convención de mensajería:
- El servicio productor declara el `exchange` de tipo `topic` de su dominio.
- Cada servicio consumidor declara su propia `queue` y el `binding` contra la `routing key`.
- Code Execution consume solicitudes y publica dos hitos: inicio y finalización.

### **A. Evento Consumido (Entrante)**

**`SolutionExecutionRequestedEvent` (Desde Challenges & Solutions Service)**
Dispara el pipeline completo de validación, construcción de script, ejecución en sandbox y persistencia de resultados.

- Exchange owner: `Challenges & Solutions`
- Queue owner: `Code Execution`
- Routing key: `challenges.solution.execution.requested`
- Manejo esperado: crear `ExecutionJob` (si no existe por `eventId`), publicar evento de inicio, ejecutar todos los tests y publicar evento de finalización.

```json
{
	"eventId": "uuid-evento-21",
	"eventType": "SolutionExecutionRequestedEvent",
	"timestamp": "2026-04-11T15:00:00Z",
	"data": {
		"solutionId": "uuid-solution-123",
		"language": "python",
		"entryFunctionName": "sumar",
		"code": "def sumar(a, b):\n    return a + b",
		"testCases": [
			{
				"testId": "test-public-1",
				"input": "2,3",
				"expectedOutput": "5",
				"isHidden": false
			},
			{
				"testId": "test-hidden-2",
				"input": "100,-50",
				"expectedOutput": "50",
				"isHidden": true
			}
		]
	}
}
```

### **B. Eventos Publicados (Salientes)**

**1. `SolutionExecutionStartedEvent` (Hacia Challenges & Solutions Service)**
Se publica cuando el job pasa de `QUEUED` a `STARTED`.

- Exchange owner: `Code Execution`
- Queue owner: `Challenges & Solutions`
- Routing key: `codeexecution.solution.execution.started`
- Manejo esperado: cambiar estado de la solución a `EXECUTING` y reflejar progreso en UI.

```json
{
	"eventId": "uuid-evento-22",
	"eventType": "SolutionExecutionStartedEvent",
	"timestamp": "2026-04-11T15:00:01Z",
	"data": {
		"solutionId": "uuid-solution-123",
		"executionId": "uuid-execution-999",
		"startedAt": "2026-04-11T15:00:01Z"
	}
}
```

**2. `SolutionExecutionCompletedEvent` (Hacia Challenges & Solutions y Code Analysis)**
Se publica al finalizar el procesamiento de todos los tests o al detectar error global.

- Exchange owner: `Code Execution`
- Queue owner: `Challenges & Solutions` y `Code Analysis`
- Routing key: `codeexecution.solution.execution.completed`
- Manejo esperado: Challenges actualiza estado funcional y Code Analysis genera feedback IA.

```json
{
	"eventId": "uuid-evento-23",
	"eventType": "SolutionExecutionCompletedEvent",
	"timestamp": "2026-04-11T15:00:02Z",
	"data": {
		"solutionId": "uuid-solution-123",
		"executionId": "uuid-execution-999",
		"attemptId": "uuid-attempt-456",
		"challengeId": "uuid-challenge-789",
		"userId": "uuid-user-000",
		"code": "def sumar(a, b):\n    return a + b",
		"isSuccessful": false,
		"totalExecutionTimeMs": 42,
		"globalError": null,
		"testResults": [
			{
				"testId": "test-public-1",
				"passed": true,
				"isHidden": false,
				"actualOutput": "5",
				"expectedOutput": "5",
				"executionTimeMs": 8,
				"errorMessage": null
			},
			{
				"testId": "test-hidden-2",
				"passed": false,
				"isHidden": true,
				"actualOutput": "49",
				"expectedOutput": "50",
				"executionTimeMs": 6,
				"errorMessage": null
			}
		]
	}
}
```

---

## **6. Catálogo de Templates por Lenguaje**

Cada lenguaje habilitado debe tener una plantilla de ejecución versionada en `LanguageTemplate`.

### **A. Python (`python`)**

```python
# {{USER_CODE}}
# parse_args es helper inyectado por el runner del sandbox

def _normalize(value: str) -> str:
		return value.replace("\r\n", "\n").rstrip()

if __name__ == "__main__":
		args = """{{TEST_INPUT}}"""
		expected = """{{EXPECTED_OUTPUT}}"""
		actual = {{ENTRY_FUNCTION_NAME}}(*parse_args(args))
		print(_normalize(str(actual)))
```

### **B. JavaScript (`javascript`)**

```javascript
// {{USER_CODE}}
// parseArgs es helper inyectado por el runner del sandbox

function normalize(value) {
	return String(value).replace(/\r\n/g, "\n").trimEnd();
}

const testInput = `{{TEST_INPUT}}`;
const expected = `{{EXPECTED_OUTPUT}}`;
const actual = globalThis["{{ENTRY_FUNCTION_NAME}}"](...parseArgs(testInput));
process.stdout.write(normalize(actual));
```

### **C. Java (`java`)**

```java
import java.util.*;

// {{USER_CODE}}
// parseArgs es helper inyectado por el runner del sandbox

public class Main {
	static String normalize(String value) {
		return value.replace("\r\n", "\n").replaceAll("\\s+$", "");
	}

	public static void main(String[] args) {
		String input = "{{TEST_INPUT}}";
		String expected = "{{EXPECTED_OUTPUT}}";
		String actual = {{ENTRY_FUNCTION_NAME}}(parseArgs(input));
		System.out.print(normalize(actual));
	}
}
```

### **D. C++ (`cpp`)**

```cpp
#include <bits/stdc++.h>
using namespace std;

// {{USER_CODE}}
// parseArgs es helper inyectado por el runner del sandbox

string normalize(string value) {
		while (!value.empty() && (value.back() == '\n' || value.back() == '\r' || value.back() == ' ' || value.back() == '\t')) {
				value.pop_back();
		}
		return value;
}

int main() {
		string input = R"({{TEST_INPUT}})";
		string expected = R"({{EXPECTED_OUTPUT}})";
		auto args = parseArgs(input);
		string actual = {{ENTRY_FUNCTION_NAME}}(args);
		cout << normalize(actual);
		return 0;
}
```

### **Política de Versionado de Templates**

- Un cambio rompiente de un template requiere incrementar `version`.
- `ExecutionJob` debe persistir la versión exacta usada para reproducibilidad.
- Si un lenguaje se deshabilita, no se aceptan nuevas ejecuciones para ese lenguaje hasta reactivarlo.
