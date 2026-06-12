use serde::Serialize;
use tauri_plugin_shell::ShellExt;
#[derive(Serialize)]
struct BackendHealth {
    name: String,
    status: String,
    mode: String,
}

#[tauri::command]
fn backend_health() -> BackendHealth {
    BackendHealth {
        name: "whatsapp-tauri".to_string(),
        status: "ok".to_string(),
        mode: "tauri".to_string(),
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init()).plugin(tauri_plugin_shell::init())
.setup(|app| {
        let sidecar = app.shell().sidecar("backend").unwrap();
        let (mut rx, _child) = sidecar.spawn().expect("Failed to spawn sidecar");
    tauri::async_runtime::spawn(async move {
        use tauri_plugin_shell::process::CommandEvent;
        while let Some(event) = rx.recv().await {
            match event {
                CommandEvent::Stdout(line) => {
                    println!("[go] {}", String::from_utf8_lossy(&line));
                }
                CommandEvent::Stderr(line) => {
                    eprintln!("[go:err] {}", String::from_utf8_lossy(&line));
                }
                _ => {}
            }
        }
    });
        Ok(())
    })
        .invoke_handler(tauri::generate_handler![backend_health])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
