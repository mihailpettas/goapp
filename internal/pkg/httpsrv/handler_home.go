package httpsrv

import (
	"html/template"
	"net/http"
)

func (s *Server) handlerHome(w http.ResponseWriter, r *http.Request) {

    w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")

	token := s.setCSRFToken(w)

	tmpl := template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>GoApp WebSocket Client</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 1rem;
            background-color: #f5f5f5;
        }
        .container {
            display: flex;
            gap: 2rem;
            margin-top: 2rem;
        }
        .controls {
            flex: 1;
            background: white;
            padding: 1.5rem;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .output-container {
            flex: 2;
            background: white;
            padding: 1.5rem;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .button-group {
            display: flex;
            gap: 1rem;
            margin-bottom: 1rem;
        }
        button {
            padding: 0.5rem 1rem;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 1rem;
            transition: background-color 0.2s;
        }
        button:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }
        #open {
            background-color: #4CAF50;
            color: white;
        }
        #close {
            background-color: #f44336;
            color: white;
        }
        #send {
            background-color: #2196F3;
            color: white;
        }
        #output {
            height: 70vh;
            overflow-y: auto;
            padding: 1rem;
            border: 1px solid #ddd;
            border-radius: 4px;
            background: #fafafa;
            font-family: monospace;
        }
        .message {
            padding: 0.5rem;
            margin: 0.25rem 0;
            border-radius: 4px;
        }
        .message.sent {
            background-color: #e3f2fd;
        }
        .message.received {
            background-color: #f1f8e9;
        }
        .message.error {
            background-color: #ffebee;
        }
        .hex-value {
            font-family: monospace;
            color: #2196F3;
            font-weight: bold;
        }
        .status {
            font-size: 0.875rem;
            margin-top: 1rem;
            padding: 0.5rem;
            border-radius: 4px;
            background: #e8f5e9;
        }
        #connection-status {
            margin-bottom: 1rem;
            padding: 0.5rem;
            border-radius: 4px;
            text-align: center;
        }
        .connected {
            background-color: #e8f5e9;
            color: #2e7d32;
        }
        .disconnected {
            background-color: #ffebee;
            color: #c62828;
        }
    </style>
</head>
<body>
    <h1>GoApp WebSocket Client</h1>
    <div class="container">
        <div class="controls">
            <div id="connection-status" class="disconnected">
                Disconnected
            </div>
            <div class="button-group">
                <button id="open">Connect</button>
                <button id="close" disabled>Disconnect</button>
            </div>
            <form id="reset-form" onsubmit="return false;">
                <button id="send" disabled>Reset Counter</button>
            </form>
            <div class="status">
                Click "Connect" to start receiving hex values. 
                Use "Reset Counter" to restart the counter from 0.
            </div>
        </div>
        <div class="output-container">
            <h2>Messages</h2>
            <div id="output"></div>
        </div>
    </div>

    <script>
    window.addEventListener("load", function(evt) {
        const output = document.getElementById("output");
        const openBtn = document.getElementById("open");
        const closeBtn = document.getElementById("close");
        const sendBtn = document.getElementById("send");
        const statusDiv = document.getElementById("connection-status");
        let ws;

        function updateConnectionStatus(connected) {
            statusDiv.textContent = connected ? "Connected" : "Disconnected";
            statusDiv.className = connected ? "connected" : "disconnected";
            openBtn.disabled = connected;
            closeBtn.disabled = !connected;
            sendBtn.disabled = !connected;
        }

        function print(type, message) {
            const msgDiv = document.createElement("div");
            msgDiv.className = "message " + type;
            msgDiv.textContent = message;
            output.appendChild(msgDiv);
            output.scrollTop = output.scrollHeight;
        }

        function formatResponse(data) {
            try {
                const response = JSON.parse(data);
                return `Iteration: ${response.iteration}, Hex Value: `+
                       `<span class="hex-value">${response.value}</span>`;
            } catch (e) {
                return data;
            }
        }

        openBtn.onclick = function(evt) {
            if (ws) {
                return false;
            }
            ws = new WebSocket({{.wsURL}});

            ws.onopen = function(evt) {
                print("sent", "Connection established");
                updateConnectionStatus(true);
            }

            ws.onclose = function(evt) {
                print("sent", "Connection closed");
                updateConnectionStatus(false);
                ws = null;
            }

            ws.onmessage = function(evt) {
                const msgDiv = document.createElement("div");
                msgDiv.className = "message received";
                msgDiv.innerHTML = formatResponse(evt.data);
                output.appendChild(msgDiv);
                output.scrollTop = output.scrollHeight;
            }

            ws.onerror = function(evt) {
                print("error", "ERROR: " + evt.data);
            }

            return false;
        };

        closeBtn.onclick = function(evt) {
            if (!ws) {
                return false;
            }
            ws.close();
            return false;
        };

        sendBtn.onclick = function(evt) {
            if (!ws) {
                return false;
            }
            print("sent", "Resetting counter");
            ws.send("{}");
            return false;
        };

        // Add CSRF token to all requests
        const csrfToken = {{.csrfToken}};
        if (csrfToken) {
            const headers = new Headers({
                'X-CSRF-Token': csrfToken
            });
        }
    });
    </script>
</body>
</html>
`))

	data := struct {
		wsURL     template.JS
		csrfToken template.JS
	}{
		wsURL:     template.JS(`"ws://" + window.location.host + "/goapp/ws"`),
		csrfToken: template.JS(`"` + token + `"`),
	}

	if err := tmpl.Execute(w, data); err != nil {
		s.error(w, http.StatusInternalServerError, err)
		return
	}
}