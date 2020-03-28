## Proxmox Spice Launcher
This program allows you to easily launch a Virtual Machine with a SPICE display.  
Once compiled, execute the script with the VM ID as an argument.

#### Example Config
There is a sample file named `.pve.example` that contains the correct layout.  
Fill out your settings and rename it to .pve

#### Release Mouse Issues
When running the script, press CTRL+ALT+R to release your mouse.  
If you would like to have CTRL+ALT be the release keys, comment out `"release-cursor=%s\n"+` and remove `spiceConfig.Cursor` from the format.