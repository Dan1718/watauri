use serde::Serialize;

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
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![backend_health])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
