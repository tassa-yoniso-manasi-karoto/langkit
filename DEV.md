## Feature selection to internal mode mapping

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
