# OGS — matriz de adopción del estudio completo

Este documento consolida **qué capacidades necesita un estudio OGS**, **quién es responsable de cada una** y **qué se reutiliza, se adapta o se construye**. Ordena la dirección completa del producto —crear un juego desde cero, invocar una fase concreta y cambiar o reparar un juego existente— en decisiones de adopción con evidencia y prioridad.

**No es una autorización de ejecución ni una matriz de compatibilidad probada.** Una fila con decisión «construir» no significa que exista implementación: significa que OGS debe producirla. La compatibilidad real de versiones se registra aparte, en la plantilla pendiente de [`docs/ogs-runtime-distribution.md`](ogs-runtime-distribution.md).

## Leyenda de decisiones y evidencia

| Etiqueta | Significado |
|---|---|
| **REUTILIZAR** | Usar una capacidad ya resuelta por Pi, Gentle Shell o el runtime nativo de Gentle AI. |
| **ADAPTAR** | Traducir una práctica de dominio de CCGS al entorno OGS, sin copiar dependencias exclusivas de Claude. |
| **CONSTRUIR** | Capacidad de producto que OGS debe crear; puede estar propuesta o inexistente. |
| **PRESERVAR** | Se mantiene sin migrar por deducción (por ejemplo, el generador legacy). |
| **NO ADOPTAR** | Se descarta explícitamente para evitar duplicación o dependencia indebida. |
| **PENDIENTE** | Sin evidencia suficiente para decidir o verificar. |

Evidencia: **EXISTENTE** = observado en fuentes de este repositorio; **INVESTIGADO** = hallazgo de las copias de estudio fijadas por SHA; **PROPUESTO** = recomendación aún sin implementación ni prueba.

## Dos entradas, el mismo estudio

El producto no es solo «cambiar una mecánica». Los tres recorridos conviven y comparten orquestación:

1. **Crear desde cero** — de la idea a un juego jugable. Es una ruta de primera clase, no un anexo futuro.
2. **Invocar una fase concreta** — entrar por diseño, arte o implementación sin recorrer todo el estudio.
3. **Cambiar o reparar** — modificar un juego existente **sin exigir un onboarding completo**.

La creación desde cero y el cambio directo usan la **misma** orquestación, persistencia y compuertas; cambia el punto de entrada, no el motor de trabajo. El primer corte demuestra el cambio directo; la ruta desde cero se planifica por etapas y **no** está implementada ni autorizada por este lote.

## Matriz de capacidades

| Capacidad | Proveedor / responsable | Decisión | Evidencia y límites actuales | Prioridad incremental |
|---|---|---|---|---|
| Concepto inicial, pilares, player fantasy y core loop | CCGS como contrato de dominio + dirección humana | ADAPTAR | Prácticas leídas en el catálogo de fases de CCGS `984023ddac0d5e27624f2baacde6105e45de375f`; **no** ejecutadas ni portadas. | Corte 2 (base del recorrido desde cero) |
| Mecánicas y narrativa opcional | CCGS (dominio) + OGS (dirección) | ADAPTAR | La narrativa puede ser mínima o **inexistente** cuando el juego lo requiere. | Corte 2 |
| GDD ligero aprobado | CCGS (mapeo diseño→GDD) + OpenSpec | ADAPTAR | CCGS mapea diseño → GDD → prerrequisitos de historia/implementación. Sin artefacto OGS generado todavía. | Corte 2 |
| Dirección de arte y assets | Fuentes externas + OGS (procedencia) | REUTILIZAR (externo) + CONSTRUIR (registro) | Los assets externos son una **ruta válida de primera clase**, no una excepción. Falta política de procedencia y licencias por ítem. | Corte 3 |
| Orquestación, fases y roles de estudio | CCGS (referencia de dominio) | ADAPTAR | Mapa de fases y roles investigado; `director-gates.md` solo se leyó parcialmente. No es un port implementado. | Corte 2 |
| Compuertas creativas asesoría vs. STOP técnico | CCGS (dominio) | ADAPTAR | Se distinguen compuertas creativas de precondiciones técnicas; `/start` recomienda sin ejecutar la fase siguiente. La aprobación creativa **no** concede permisos de instalación o ejecución. | Corte 2 |
| Persistencia de decisiones y evidencia | OpenSpec + Engram | REUTILIZAR + ADAPTAR | OpenSpec conserva artefactos; Engram es companion separado, no lo incluye Gentle Shell. | Corte 1 |
| Orquestación, delegación y seguimiento | Pi (host) + Gentle Shell | REUTILIZAR | ODD por defecto, delegación, perfiles y barrera de revisión. Es un **seam de reutilización**, no motivo para un segundo runner. | Corte 1 |
| Carga de skills y paquetes de estudio | Pi (host) | REUTILIZAR | El paquete Pi publica `./skills/ogs-godot-change` y `./skills/ogs-core`; carga normal de Pi, sin `--link` obligatorio. `private: true` es un flag del paquete npm, no una decisión sobre la visibilidad del repositorio. | Corte 1 |
| Verificación técnica (lógica/estructura) | OGS + Gentle Shell (rol verificador) | REUTILIZAR + CONSTRUIR | Suite `tests/pi-godot-change.test.mjs` EXISTENTE; `testdata/godot-minimal-2d` como fixture. | Corte 1 |
| Ejecución headless de Godot | Godot 4 vía `GODOT_BIN` aprobado | REUTILIZAR | El script `verify_mechanic.gd` y el runner ya existen. La ausencia de `godot` en `PATH` **no** prueba ausencia en la máquina. Histórico: una corrida 2D headless y un playtest humano existieron por separado, pero el corte integrado de este documento sigue sin ejecutarse. | Corte 1 |
| Playtesting humano (diversión, feel visual) | Persona (dirección creativa) | ADAPTAR (propiedad humana) | La verificación headless **no** evalúa diversión ni calidad visual; requiere juicio humano. | Corte 1 (dentro del corte) |
| Cambio y reparación directa de mecánica | OGS (`ogs-godot-change`) | CONSTRUIR (parcial) | Seam de skill EXISTENTE para un cambio acotado; **no** es todavía el flujo integrado de conversación. | Corte 1 |
| Implementación desde cero (nuevo juego) | OGS + Godot | CONSTRUIR | El recorrido completo idea→juego jugable sigue **propuesto**; el backend aceptado del instalador/wizard (detección, plan, consentimiento) no implementa la creación desde cero. | Corte 2 |
| Revisión nativa (RDD) y autoridad | Runtime nativo Gentle AI vía Gentle Shell | REUTILIZAR | Frontera de autoridad que OGS **no** redefine. Interioridades no auditadas a fondo. | Corte 1 |
| Distribución, versión validada y actualización | Producto OGS | CONSTRUIR (parcial) | El backend del instalador (detección / plan / consentimiento / ejecución) está aceptado en alcance **fuente y fixture** (G1–6); la instalación o reutilización real en un objetivo Linux/WSL autorizado (G7) sigue **pendiente**. El código fuente se prepara para un repositorio público, mientras el paquete npm conserva `private: true`. Ver [distribución](ogs-runtime-distribution.md). | Corte 3 |
| Generador legacy Go/OpenCode | OGS | PRESERVAR | Se conserva y no se migra por deducción durante esta dirección Pi-first. | Posterior |
| Segundo orquestador propio de OGS | — | NO ADOPTAR | Duplicaría ODD, delegación, revisión y tracking que ya aporta Gentle Shell. | — |
| Mecanismos exclusivos de Claude (hooks, tiers de modelos, conteo de agentes) | — | NO ADOPTAR | Acoplarían OGS a un entorno distinto; se adapta el **valor de dominio**, no la dependencia. | — |

## Qué existe hoy y qué es solo práctica propuesta

**EXISTENTE en este repositorio**

- Generador legacy en Go: wizard que **genera** config de workspace, `GAME-STUDIO.md`, perfil y artefacto JSON; la selección se marca como metadata-only y el artefacto final no instala dependencias por sí mismo.
- Backend del instalador (detección / plan / consentimiento / ejecución) aceptado en alcance fuente y fixture; la instalación real sigue pendiente (G7).
- Paquete Pi companion con `private: true` (paquete npm privado) que publica las skills `./skills/ogs-godot-change` y `./skills/ogs-core`. **Ojo:** `private: true` describe el paquete npm, **no** la visibilidad del repositorio Git; el código fuente de OGS se prepara para un repositorio público.
- Fixture `testdata/godot-minimal-2d/` y suite `tests/pi-godot-change.test.mjs` con verificación headless de movimiento.

**INVESTIGADO / PROPUESTO (sin implementar en OGS)**

- Flujo de estudio completo desde cero: concepto, pilares, core loop, GDD ligero, arte, playtesting e iteración.
- Compuertas creativas de CCGS y su mapeo diseño→GDD→implementación, traducidos a Pi/Gentle Shell.
- Instalador que reutilice el entorno compatible y pida permiso explícito antes de cambiar.

**Distinción obligatoria:** que una práctica esté documentada en una referencia **no** la convierte en capacidad de OGS. La columna de evidencia marca cada fila como existente, investigada o propuesta.

## Fronteras de responsabilidad

| Capa | Responsabilidad | No hace |
|---|---|---|
| Producto OGS | Dominio de estudio, dirección de producto, contratos de dominio y su empaquetado Pi. | No construye un segundo orquestador ni redefine autoridad nativa. |
| Host Pi | Sesiones, carga de paquetes, skills, base de agente, modelo y autenticación. | No posee el dominio de juego. |
| Gentle Shell | ODD por defecto, subagentes, seguimiento, perfiles y barrera de revisión. | No es sandbox; su cobertura de guardas es limitada. |
| Runtime nativo Gentle AI | Autoridad de revisión (RDD) invocada por Gentle Shell. | No se reimplementa ni se migra desde OGS. |
| CCGS | Referencia de **dominio**: fases, roles, artefactos y compuertas creativas. | No es dependencia de runtime obligatoria ni un port ya hecho. |

## Hoja de ruta por cortes

| Corte | Objetivo | Estado | Entrada |
|---|---|---|---|
| **Corte 1 — cambio de velocidad** | Demostrar un cambio acotado en una **copia nueva** del fixture Godot existente, con aprobación humana y evidencia objetiva. | Plan de prueba **propuesto** en [`docs/ogs-godot-speed-slice.md`](ogs-godot-speed-slice.md); **sin ejecutar**. Existen una corrida de motor Godot 2D y un playtest humano como evidencia histórica, pero **no** ejecutan ni aceptan este flujo integrado. | Cambio directo |
| **Corte 2 — juego desde cero** | Recorrer idea → pilares/core loop → GDD ligero → implementación mínima → verificación y playtest humano → **iteración tras el playtest**. | **No implementado ni autorizado** por este lote. | Creación desde cero |
| **Corte 3 — arte, assets y distribución** | Ruta de assets externos con procedencia, e instalador con aviso y consentimiento. | Propuesto para arte y assets externos; el backend Go del instalador está aceptado en alcance **fuente y fixture**, y la instalación o reutilización real (G7) sigue **pendiente**. | Transversal |

El Corte 1 valida el **flujo de trabajo** (conversación, compuerta, delegación y evidencia), no la calidad del juego. El Corte 2 reutiliza exactamente esa orquestación para la **creación desde cero** y añade las fases creativas (concepto, pilares, core loop y GDD ligero), la implementación mínima, el playtest humano y la **iteración posterior al playtest**. El arte, los assets externos y la distribución quedan en el **Corte 3**, como marcan las filas de la matriz y esta misma hoja de ruta.

## Límites de esta matriz

- **No es una matriz de compatibilidad probada de runtime.** Es un mapa de adopción y propiedad.
- Una decisión de reutilización **no** valida una combinación concreta de versiones; eso es evidencia observada aparte.
- La adopción de assets o dependencias de terceros requiere **revisión por ítem** de licencia y procedencia; la licencia raíz MIT no sustituye esa revisión.
- No implica endorsement de marca ni de proyecto alguno.

## Fuentes y atribución

**Leído directamente en este repositorio:** `README.md`, `package.json`, `openspec/config.yaml`, `skills/ogs-godot-change/SKILL.md`, `skills/ogs-godot-change/references/godot-setup.md`, `testdata/godot-minimal-2d/`, `tests/pi-godot-change.test.mjs` y [`docs/ogs-runtime-distribution.md`](ogs-runtime-distribution.md).

**Investigación previa provista por el coordinador (no re-ejecutada aquí):** catálogo de fases y compuertas de CCGS en [`catalog` `984023ddac0d5e27624f2baacde6105e45de375f`](https://github.com/Donchitos/Claude-Code-Game-Studios/tree/984023ddac0d5e27624f2baacde6105e45de375f) (`.claude/docs/workflow-catalog.yaml`); hallazgos de Gentle Shell sobre ODD/TDD/RDD y guardas; candidato público Gentle Shell 3.5.1 en `df41b3a2420f8f9910cebd8b4bfb4a29f5fefa60` y runtime [`gentle-shell-launcher.mjs`](https://github.com/Gentleman-Programming/gentle-shell/blob/df41b3a2420f8f9910cebd8b4bfb4a29f5fefa60/runtime/gentle-shell-launcher.mjs). Estas filas son **investigación de fuente**, no ejecución local.

## Siguiente paso

El Corte 1 está especificado en [`docs/ogs-godot-speed-slice.md`](ogs-godot-speed-slice.md). Antes de cualquier ejecución se necesita autorización separada y explícita; este documento no la concede.
