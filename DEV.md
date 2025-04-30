## Feature selection to internal mode mapping

Feature selection must be 'translated' into a Task mode. These modes ***for the most part*** correspond to CLI subcommands.

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
    <th>enhance</th>
  </tr></thead>
<tbody>
  <tr>
    <td>Make a merged video</td>
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
    <td>✅</td>
  </tr>
  <tr>
    <td>Make tokenized <var>subtitles</var></td>
    <td>🔳</td>
    <td>🚫</td>
    <td>✅</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make translit <var>subtitles</var></td>
    <td>🔳</td>
    <td>🚫</td>
    <td>✅<br></td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make tokenized <var>dubtitles</var></td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🚫<br></td>
    <td>🚫</td>
  </tr>
  <tr>
    <td>Make translit <var>dubtitles</var></td>
    <td>🔳</td>
    <td>🔳</td>
    <td>🚫<br></td>
    <td>🚫</td>
  </tr>
  <tr>
    <td>Make dubtitles</td>
    <td>🔳</td>
    <td>✅</td>
    <td>❌</td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make condensed audio</td>
    <td>🔳</td>
    <td>❌</td>
    <td>❌<br></td>
    <td>❌</td>
  </tr>
  <tr>
    <td>Make Anki notes<br></td>
    <td>✅</td>
    <td>❌</td>
    <td>❌</td>
    <td>❌</td>
  </tr>
</tbody></table>
