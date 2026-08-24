Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()
$form=New-Object Windows.Forms.Form; $form.Text='ImageRelayWorker'; $form.Size=New-Object Drawing.Size(520,300); $form.StartPosition='CenterScreen'
$labels=@('URL de la API','Token','ID del worker'); $props=@('API_BASE_URL','WORKER_TOKEN','WORKER_ID'); $boxes=@()
for($i=0;$i -lt 3;$i++){ $l=New-Object Windows.Forms.Label; $l.Text=$labels[$i]; $l.Location=New-Object Drawing.Point(20,(25+$i*55)); $l.AutoSize=$true; $form.Controls.Add($l); $b=New-Object Windows.Forms.TextBox; $b.Location=New-Object Drawing.Point(150,(20+$i*55)); $b.Size=New-Object Drawing.Size(330,25); if($i -eq 1){$b.UseSystemPasswordChar=$true}; $form.Controls.Add($b); $boxes+=$b }
$button=New-Object Windows.Forms.Button; $button.Text='Guardar y activar'; $button.Location=New-Object Drawing.Point(150,195); $button.Size=New-Object Drawing.Size(180,35); $form.Controls.Add($button)
$button.Add_Click({ if($boxes[0].Text -notmatch '^https?://|^$' -or !$boxes[1].Text -or !$boxes[2].Text){[Windows.Forms.MessageBox]::Show('Completa URL, token e ID.');return}; for($i=0;$i -lt 3;$i++){[Environment]::SetEnvironmentVariable($props[$i],$boxes[$i].Text,'Machine')}; Start-Service ImageRelayWorker -ErrorAction SilentlyContinue; [Windows.Forms.MessageBox]::Show('Configuración guardada. Servicio activado.'); $form.Close() })
[void]$form.ShowDialog()
