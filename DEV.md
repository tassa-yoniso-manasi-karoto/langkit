## Target Versions

Go 1.23.4 <br>
Toolchain 1.23.9 (⚠️) <br>
Wails CLI 2.9.0 (⚠️) <br>
Wails modules 2.10.1 or latest v2 <br>

> [!NOTE]
> ⚠️ = most recent version supported by [wails-action](https://github.com/dAppServer/wails-build-action), newer version will fail the build process due to CGo conflicts.

> [!WARNING]
> The TOOLCHAIN IS AUTORITATIVE FOR THE GO VERSION USED IN THE GH ACTION. <br>You **MUST use go 1.23** for `go mod tidy` otherwise the toolchain will be overwritten to a newer version. <br> <br>
> In other words, even if go1.23 is specified as go version, the **GH action will use the version specified by the toolchain for the build process** and thus it will fail. (I tried with GOTOOLCHAIN=local but even that got ignored).

## Feature(s) selection to internal mode matrix

Feature selection must be 'translated' into a Task mode. These modes ***for the most part*** correspond to CLI subcommands.

<table><thead>
  <tr>
    <th>requires..-</th>
    <th>sub?</th>
    <th>lang?</th>
  </tr></thead>
<tbody>
  <tr>
    <td>Make a merged video</td>
    <td>NO</td>
    <td>NO</td>
  </tr>
  <tr>
    <td>Make enhanced track</td>
    <td>NO</td>
    <td>opt</td>
  </tr>
  <tr>
    <td>Make condensed audio</td>
    <td>yes</td>
    <td>rather</td>
  </tr>
  <tr>
    <td>Make tokenized subtitle</td>
    <td>yes</td>
    <td>yes</td>
  </tr>
  <tr>
    <td>Make translit subtitle</td>
    <td>yes</td>
    <td>yes</td>
  </tr>
  <tr>
    <td>Make tokenized dubtitle</td>
    <td>yes</td>
    <td>yes</td>
  </tr>
  <tr>
    <td>Make translit dubtitle</td>
    <td>yes</td>
    <td>yes</td>
  </tr>
  <tr>
    <td>Make dubtitle</td>
    <td>yes</td>
    <td>yes</td>
  </tr>
  <tr>
    <td>Make Anki notes<br></td>
    <td>yes</td>
    <td>rather</td>
  </tr>
</tbody>
</table>

✅ = default behavior

🔳 = optionally available

❌ = not available

🚫 = not applicable

<table><thead>
  <tr>
    <th><sub>↓ GUI selected</sub>   ╲       <sup>tsk.Mode →</sup></th>
    <th>subs2cards</th>
    <th>subs2dubs</th>
    <th>translit</th>
    <th>condense</th>
    <th>enhance</th>
  </tr></thead>
<tbody>
  <tr>
    <td>Make a merged video</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🔳</td>
  </tr>
  <tr>
    <td>Make enhanced track</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>✅</td>
  </tr>
  <tr>
    <td>Make condensed audio</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🔳<br></td>
    <td>✅</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make tokenized subtitle</td>
    <td>🔳</td>
    <td>🚫</td>
    <td>✅</td>
    <td>❌</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make translit subtitle</td>
    <td>🔳</td>
    <td>🚫</td>
    <td>✅<br></td>
    <td>❌</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make tokenized dubtitle</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🚫<br></td>
    <td>❌</td>
    <td>🚫</td>
  </tr>
  <tr>
    <td>Make translit dubtitle</td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🚫<br></td>
    <td>❌</td>
    <td>🚫</td>
  </tr>
  <tr>
    <td>Make dubtitle</td>
    <td>🔳</td>
    <td>✅</td>
    <td>❌</td>
    <td>❌</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make Anki notes<br></td>
    <td>✅</td>
    <td>❌</td>
    <td>❌</td>
    <td>❌</td>
    <td>❌</td>
  </tr>
</tbody></table>
