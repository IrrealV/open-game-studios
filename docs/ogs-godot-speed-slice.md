# OGS — primer corte: cambio de velocidad en el fixture Godot

Este documento es un **plan de prueba propuesto**, no un resultado. Define un único cambio acotado —la velocidad del jugador— en una **copia nueva** del fixture Godot existente, con compuerta humana antes de escribir y evidencia objetiva de aceptación.

**Estado: NO EJECUTADO y NO AUTORIZADO.** Ningún comando de este documento se corrió. No se estableció identidad de instalación ni versión de runtime. Un PASS documental no significa que la combinación Pi/Gentle Shell/OGS funcione, que el juego sea divertido ni que exista soporte multiplataforma.

> **Nota de estado (no confundir).** Existe evidencia histórica separada: una corrida headless del motor Godot sobre el fixture 2D y un playtest humano observado. Esa evidencia pertenece al comportamiento 2D del fixture, **no** a este corte integrado: el flujo de conversación acotado de este documento **sigue sin ejecutarse**. No se debe promover la evidencia histórica 2D a aceptación del corte ni al revés.

## Alcance del corte

- **Cambio único:** `speed` del jugador en una copia autorizada de `testdata/godot-minimal-2d`.
- **Fixture original intacto:** `testdata/godot-minimal-2d/` **no** se modifica. La suite ya verifica en su hook `after` que los cuatro archivos del fixture siguen byte a byte.
- **Copy-only:** cualquier edición ocurre en una copia nueva; su ruta exacta es un enlace futuro que requiere aprobación, no algo ya verificado.
- **Fuera de alcance en todos los casos:** instalaciones, descargas, cambios de `PATH`/configuración global, `--link`, reparación del observador SDK, búsqueda de NaN y launcher nuevo. La red solo se contempla, si hiciera falta, dentro de una sesión de modelo futura y explícitamente aprobada (ver «Permisos: lote actual vs. corte futuro»).

## Permisos: lote actual vs. corte futuro

| Momento | Qué se autoriza | Qué no |
|---|---|---|
| Lote actual (documentación) | Redactar y revisar estos documentos. | Sesión de modelo de producto, red y checks de runtime: **nada de eso corre ahora**. |
| Corte integrado futuro | Nada aún: exige **aprobación explícita y separada** de la sesión de modelo prevista y de la red/consumo de proveedor que esa sesión necesite. | Ejecución automática de modelos/API y cualquier inventario privado nuevo. |

Los checks de Godot locales (headless o suite Node) **no** requieren modelo ni red. Instalaciones, descargas, cambios de `PATH` o configuración global, `--link`, reparación del SDK y diagnósticos de NaN quedan fuera del corte en todos los casos.

## De dónde salen los números (fuentes reales)

Valores derivados de las fuentes leídas, **no** de constantes históricas asumidas:

| Dato | Fuente | Valor |
|---|---|---|
| Velocidad base | `testdata/godot-minimal-2d/player.gd` | `@export var speed: float = 120.0` |
| Frames medidas | `testdata/godot-minimal-2d/verify_mechanic.gd` | `FRAME_COUNT := 12` |
| Delta fijo | `verify_mechanic.gd` | `FIXED_DELTA := 1.0 / 60.0` |
| Tolerancia | `verify_mechanic.gd` | `DISTANCE_TOLERANCE := 0.001` |
| Fórmula esperada | `verify_mechanic.gd` | `expected = speed * FIXED_DELTA * FRAME_COUNT` |
| Desplazamiento base | derivado: `120 * (1/60) * 12` | `24` |
| Velocidad objetivo | `tests/pi-godot-change.test.mjs` (prueba de copia temporal) | `240.0` |
| Desplazamiento objetivo | derivado: `240 * (1/60) * 12` | `48` |

La suite existente ya reemplaza `@export var speed: float = 120.0` por `@export var speed: float = 240.0` en una copia temporal y espera `48`. Este corte **no** vuelve a probar esa unidad: propone el **flujo integrado de conversación** que la suite por sí sola no demuestra.

## Precondiciones

1. Copia del fixture creada y aprobada por la persona; ruta exacta declarada antes de escribir.
2. Godot 4 disponible por un `GODOT_BIN` **explícito y aprobado**, confirmado con `<GODOT_BIN> --headless --version`. La ausencia de `godot` en `PATH` **no** prueba ausencia en la máquina.
3. Aprobación humana explícita del plan exacto (mecánica, ruta, edición, checks y límites) **antes** de tocar la copia.
4. Sin instalación, descarga, cambio de `PATH` ni configuración global. La ejecución de un modelo de producto, la red y el consumo de proveedor pertenecen al corte futuro y exigen aprobación explícita y separada **antes** de correr (ver «Permisos: lote actual vs. corte futuro»).
5. Fixture original preservado; sin cleanup destructivo oculto.

Si Godot es inusable, la versión no es 4.x, o la ruta no está aprobada, el corte **se detiene**; no se instala ni se sustituye el binario.

## Dos recorridos distintos (no confundir)

| | Recorrido integrado (objetivo del corte) | Suite existente (evidencia de soporte) |
|---|---|---|
| Qué prueba | Que Pi + Gentle Shell + OGS conversan: activan la skill, presentan plan, respetan la compuerta y delegan. | Que el harness, el fixture y la mecánica seleccionada se comportan como se espera. |
| Entrada | Sesión de Pi cargando el paquete OGS de forma normal, invocando `ogs-godot-change`. | `node --test` y Godot headless directo. |
| Evidencia | Plan presentado, aprobación registrada, edición en la copia, checks y playtest humano. | Salida `OGS_MEASUREMENT`, exit codes y resultado de la suite. |
| Límite | No ejecutado todavía; depende de una identidad de runtime no establecida. | **No** prueba la conversación del agente ni la calidad de juego. |

Correr la suite existente **no** equivale a completar el recorrido integrado, y viceversa.

## Compuerta humana antes del cambio

Secuencia obligatoria:

1. La persona recibe el plan (mecánica, ruta de copia, edición de una línea, checks, límites y cleanup).
2. La persona **aprueba o rechaza** explícitamente. Silencio o plan incompleto = sin escritura.
3. Solo tras la aprobación, un único escritor aplica la edición en la copia.
4. El verificador corre los checks y reporta evidencia observada.
5. La persona playtestea y decide si la sensación es aceptable.

La aprobación creativa del cambio **no** concede permisos de instalación, red, configuración o ejecución más amplia.

## Comandos FUTUROS (derivados, NO EJECUTADOS)

Salen del README, de `tests/pi-godot-change.test.mjs` y de `verify_mechanic.gd`. Son **propuestas**; no se corrieron.

```sh
# FUTURO / NO EJECUTADO — chequeo estructural (Godot no requerido)
node --test tests/pi-godot-change.test.mjs

# FUTURO / NO EJECUTADO — suite completa con Godot 4 aprobado
# Nota: la asignación GODOT_BIN=... vale solo para ESTE proceso Node; no queda definida después.
GODOT_BIN="/absolute/path/to/godot4" node --test tests/pi-godot-change.test.mjs

# FUTURO / NO EJECUTADO — verificación directa sobre la copia aprobada
# Usa directamente la ruta del binario aprobado (mismo placeholder), no una variable heredada.
"/absolute/path/to/godot4" --headless --path "<COPIA-APROBADA>" --script res://verify_mechanic.gd -- --expected-speed=240
```

Advertencias que este plan **no oculta**:

- `<COPIA-APROBADA>` es un enlace futuro; su ruta no está verificada ni aprobada.
- `GODOT_BIN=/… node …` define la variable **solo** para ese proceso Node. Por eso la verificación directa escribe la ruta del binario aprobado de forma explícita y **no** depende de una variable ya extinguida.
- La suite Node crea copias temporales y **no las elimina** por sí sola. Cualquier limpieza es una acción separada, explícita y destructiva que requiere su propia aprobación.
- Ningún comando instala, descarga, cambia `PATH` ni edita configuración global.
- Reemplazar `<COPIA-APROBADA>` por el fixture original violaría el alcance y está prohibido.

## Evidencia de aceptación exacta

**Estado: NO EJECUTADA.** Ninguna evidencia de esta sección fue observada todavía. Las subsecciones separan lo que **acepta el corte** de lo que solo **soporta** la decisión; todas describen resultados **previstos (futuros)**, no observados.

### Aceptación del corte (debe verificar la COPIA aprobada)

La aceptación exige confirmar directamente la copia editada, no una copia temporal del harness.

| Check | Evidencia prevista |
|---|---|
| Diff/origen de la copia | En la copia aprobada, **solo** cambia el default de `player.gd`: `@export var speed: float = 120.0` → `@export var speed: float = 240.0`. Nada más. |
| Archivos sin tocar en la copia | `player.tscn` y `verify_mechanic.gd` idénticos a los del fixture original: sin overrides de velocidad en la escena y sin cambios en el verificador. |
| Fixture original preservado | `testdata/godot-minimal-2d/` sin cambios (los cuatro archivos). La copia conserva su directorio temporal; sin cleanup destructivo oculto. |
| Verificación directa sobre la COPIA | El comando Godot headless apunta a la copia aprobada y observa el **default instanciado** de `player.gd` (no un override de velocidad): `status: "pass"`, `expected_speed: 240`, `measured_distance: 48`, error ≤ `0.001`, exit `0`. |

### Evidencia de soporte (suite Node sobre copias temporales propias)

La suite `tests/pi-godot-change.test.mjs` trabaja sobre **sus propias copias temporales**, no sobre la copia aprobada. Por eso **no** acepta el corte: solo demuestra que el harness, el fixture y la mecánica se comportan como se espera.

| Check de soporte | Evidencia prevista |
|---|---|
| Estructural sin Godot | 6 pruebas pasan, 4 se omiten por falta de `GODOT_BIN`; el fixture queda intacto. |
| Estructural con Godot | 10 pruebas pasan, 0 omitidas. |
| Mecánica (baseline) | `status: "pass"`, `expected_speed: 120`, `measured_distance: 24`, error ≤ `0.001`, exit `0`. |
| Mecánica (cambio) | `status: "pass"`, `expected_speed: 240`, `measured_distance: 48`, error ≤ `0.001`, exit `0`. |
| Negativo de expectativa | Expectativa `240` sobre velocidad base `120` → `status: "fail"`, exit `1`. |
| Entrada inválida | `--expected-speed=invalid` → `status: "error"`, exit `2`. |
| Preservación | `testdata/godot-minimal-2d` sin cambios (los cuatro archivos). |

### Juicio humano

| Check | Evidencia prevista |
|---|---|
| Playtest humano | Juicio humano sobre la sensación de velocidad; no automatizable ni sustituible por el headless. |

La evidencia proviene de la fuente real `verify_mechanic.gd` (bloque `OGS_MEASUREMENT` en JSON) y de los exit codes; no se inventan métricas nuevas. Un PASS de la suite de soporte **no** es aceptación del corte: la aceptación requiere el diff sobre la copia aprobada y el chequeo directo contra **esa** copia.

## Roles de delegación

| Rol | Responsabilidad | Límite |
|---|---|---|
| Persona (dirección) | Aprueba el plan, la ruta de copia y el resultado de playtest. | Decide sentimiento y aceptación; no se simula. |
| Orquestador padre | Deriva superficies de edición, mantiene un solo hilo de escritura y presenta la compuerta. | No escribe código de producto en este corte. |
| Escritor acotado | Aplica la edición de una línea **solo** en la copia aprobada. | No amplía alcance, no instala, no borra. |
| Verificador | Corre los checks autorizados y reporta resultados observados. | Solo lectura y ejecución aprobada; sin reescritura. |

## Verificación técnica vs. playtest humano

- **Técnica (headless):** mide desplazamiento y valida la mecánica seleccionada. Es evidencia de comportamiento mecánico, nada más.
- **Humana (playtest):** evalúa diversión, legibilidad y sensación de velocidad. La verificación lógica **no** la sustituye.

Un headless verde **no** promete sandboxing, ausencia universal de deriva de estado, rollback completo ni obediencia del modelo.

## Identidad, procedencia y estado compartido (checks futuros acotados)

Si el corte se autoriza, conviene registrar —como **checks futuros acotados**, no como inventario privado ni auditoría global—:

- Revisión exacta de OGS, versión de Pi, versión de Gentle Shell y pin del runtime nativo usado en la corrida.
- Cómo se cargó el paquete OGS (carga normal de Pi; sin `--link`) y si hubo deriva de estado compartido (agent home).
- Versión de Godot observada vía `--headless --version`.

Estos checks **no** están autorizados aquí y no se realizaron. La combinación publicada candidata (Gentle Shell 3.5.1 en `df41b3a2420f8f9910cebd8b4bfb4a29f5fefa60`, Pi 0.87.1 y runtime nativo Gentle AI 3.5.0 fijado por Shell) sigue siendo **pin declarado, no instalación ni compatibilidad probada**.

## Condiciones de parada

- Godot 4 ausente, de versión incorrecta o no aprobado.
- Plan, ruta de copia o edición distintos de lo aprobado.
- Cualquier necesidad de instalar, descargar, tocar `PATH` o configuración.
- Cambios inesperados en el fixture original.
- Check requerido que falla o evidencia no concluyente.
- Ambigüedad de alcance o pedido de ampliarlo.

Detenerse y reportar es el resultado correcto; no se reintenta con comandos distintos ni se amplía el alcance.

## Permisos diferidos (requieren aprobación separada)

- Instalar o actualizar Godot, Pi, Gentle Shell o el runtime nativo.
- Cualquier `--link` o cambio de configuración.
- Sesión de modelo de producto y la red/consumo de proveedor que estrictamente requiera, con alcance explícito; también cualquier investigación externa.
- Modificar el fixture original, el código de producto, tests o OpenSpec.
- Cleanup destructivo de temporales.
- Operaciones Git, staging, commits o releases.

## Lo que este corte NO demuestra

- Que la combinación exacta de runtime esté instalada o sea compatible.
- Que la conversación integrada del agente funcione (aún no ejecutada).
- Calidad de juego, diversión, arte ni exportación.
- Soporte Windows/macOS.
- Aislamiento total, rollback automático ni ausencia universal de deriva de estado.

## Checklist

- [ ] Copia autorizada creada y ruta declarada.
- [ ] `GODOT_BIN` aprobado y verificado como Godot 4.
- [ ] Plan exacto aprobado por la persona **antes** de escribir.
- [ ] Edición aplicada solo en la copia; fixture original intacto.
- [ ] Checks técnicos corridos y reportados con resultados observados.
- [ ] Playtest humano registrado como juicio aparte.
- [ ] Sin instalaciones, descargas, `--link`, cambios de `PATH`/configuración global, reparación del SDK ni diagnósticos de NaN.
- [ ] Si el corte integrado usa un modelo: sesión, red y consumo aprobados por separado **antes** de ejecutar.

## Siguiente paso

Revisión humana de este plan y autorización separada del corte. Ver también la matriz de adopción en [`docs/ogs-adoption-matrix.md`](ogs-adoption-matrix.md) y la arquitectura de distribución en [`docs/ogs-runtime-distribution.md`](ogs-runtime-distribution.md).
