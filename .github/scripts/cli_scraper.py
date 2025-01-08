import subprocess
import json
import re

def replace_angle_brackets(text):
    """
    Replace any text within angle brackets with backticks to prevent Markdown rendering issues.
    Example: "<snapshotName>" becomes "`snapshotName`"
    """
    return re.sub(r'<(.*?)>', r'`\1`', text)

def generate_anchor_id(cli_tool, command_chain):
    """
    Generate a unique anchor ID based on the entire command chain.

    Example:
        cli_tool = "avalanche"
        command_chain = ["blockchain", "create"]
        -> anchor_id = "avalanche-blockchain-create"
    """
    full_chain = [cli_tool] + command_chain
    anchor_str = '-'.join(full_chain)
    # Remove invalid characters for anchors, and lowercase
    anchor_str = re.sub(r'[^\w\-]', '', anchor_str.lower())
    return anchor_str

def get_command_structure(cli_tool, command_chain=None, max_depth=10, current_depth=0, processed_commands=None):
    """
    Recursively get a dictionary of commands, subcommands, flags (with descriptions),
    and descriptions for a given CLI tool by parsing its --help output.
    """
    if command_chain is None:
        command_chain = []
    if processed_commands is None:
        processed_commands = {}

    current_command = [cli_tool] + command_chain
    command_key = ' '.join(current_command)

    # Prevent re-processing of the same command
    if command_key in processed_commands:
        return processed_commands[command_key]

    # Prevent going too deep
    if current_depth > max_depth:
        return None

    command_structure = {
        "description": "",
        "flags": [],
        "subcommands": {}
    }

    print(f"Processing command: {' '.join(current_command)}")

    # Run `<command> --help`
    try:
        help_output = subprocess.run(
            current_command + ["--help"],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            timeout=10,
            stdin=subprocess.DEVNULL
        )
        output = help_output.stdout
        # Some CLIs return a non-zero exit code but still provide help text, so no strict check here
    except subprocess.TimeoutExpired:
        print(f"[ERROR] Timeout expired for command: {' '.join(current_command)}")
        return None
    except Exception as e:
        print(f"[ERROR] Exception while running: {' '.join(current_command)} -> {e}")
        return None

    if not output.strip():
        print(f"[WARNING] No output for command: {' '.join(current_command)}")
        return None

    # --- Extract Description ------------------------------------------------------
    description_match = re.search(r"(?s)^\s*(.*?)\n\s*Usage:", output)
    if description_match:
        description = description_match.group(1).strip()
        command_structure['description'] = replace_angle_brackets(description)

    # --- Extract Flags (including Global Flags) -----------------------------------
    flags = []
    # "Flags:" section
    flags_match = re.search(r"(?sm)^Flags:\n(.*?)(?:\n\n|^\S|\Z)", output)
    if flags_match:
        flags_text = flags_match.group(1)
        flags.extend(re.findall(
            r"^\s+(-{1,2}[^\s,]+(?:,\s*-{1,2}[^\s,]+)*)\s+(.*)$",
            flags_text,
            re.MULTILINE
        ))

    # "Global Flags:" section
    global_flags_match = re.search(r"(?sm)^Global Flags:\n(.*?)(?:\n\n|^\S|\Z)", output)
    if global_flags_match:
        global_flags_text = global_flags_match.group(1)
        flags.extend(re.findall(
            r"^\s+(-{1,2}[^\s,]+(?:,\s*-{1,2}[^\s,]+)*)\s+(.*)$",
            global_flags_text,
            re.MULTILINE
        ))

    if flags:
        command_structure["flags"] = [
            {
                "flag": f[0].strip(),
                "description": replace_angle_brackets(f[1].strip())
            }
            for f in flags
        ]

    # --- Extract Subcommands ------------------------------------------------------
    subcommands_match = re.search(
        r"(?sm)(?:^Available Commands?:\n|^Commands?:\n)(.*?)(?:\n\n|^\S|\Z)",
        output
    )
    if subcommands_match:
        subcommands_text = subcommands_match.group(1)
        # Lines like: "  create   Create a new something"
        subcommand_lines = re.findall(r"^\s+([^\s]+)\s+(.*)$", subcommands_text, re.MULTILINE)

        for subcmd, sub_desc in sorted(set(subcommand_lines)):
            sub_desc_clean = replace_angle_brackets(sub_desc.strip())
            sub_structure = get_command_structure(
                cli_tool,
                command_chain + [subcmd],
                max_depth,
                current_depth + 1,
                processed_commands
            )
            if sub_structure is not None:
                if not sub_structure.get('description'):
                    sub_structure['description'] = sub_desc_clean
                command_structure["subcommands"][subcmd] = sub_structure
            else:
                command_structure["subcommands"][subcmd] = {
                    "description": sub_desc_clean,
                    "flags": [],
                    "subcommands": {}
                }

    processed_commands[command_key] = command_structure
    return command_structure

def generate_markdown(cli_structure, cli_tool, file_path):
    """
    Generate a Markdown file from the CLI structure JSON object in a developer-friendly format.
    No top-level subcommand bullet list.
    """
    def write_section(structure, file, command_chain=None):
        if command_chain is None:
            command_chain = []

        # If at root level, do not print a heading or bullet list, just go straight
        # to recursing through subcommands.
        if command_chain:
            # Determine heading level (but max out at H6)
            heading_level = min(1 + len(command_chain), 6)

            # Build heading text:
            if len(command_chain) == 1:
                heading_text = f"{cli_tool} {command_chain[0]}"
            else:
                heading_text = ' '.join(command_chain[1:])

            # Insert a single anchor before writing the heading
            anchor = generate_anchor_id(cli_tool, command_chain)
            file.write(f'<a id="{anchor}"></a>\n')
            file.write(f"{'#' * heading_level} {heading_text}\n\n")

            # Write description
            if structure.get('description'):
                file.write(f"{structure['description']}\n\n")

            # Write usage
            full_command = f"{cli_tool} {' '.join(command_chain)}"
            file.write("**Usage:**\n")
            file.write(f"```bash\n{full_command} [subcommand] [flags]\n```\n\n")

            # If there are subcommands, list them only if we're not at the root
            # (which we aren't, because command_chain is non-empty).
            subcommands = structure.get('subcommands', {})
            if subcommands:
                file.write("**Subcommands:**\n\n")
                # Index of subcommands
                for subcmd in sorted(subcommands.keys()):
                    sub_desc = subcommands[subcmd].get('description', '')
                    sub_anchor = generate_anchor_id(cli_tool, command_chain + [subcmd])
                    file.write(f"- [`{subcmd}`](#{sub_anchor}): {sub_desc}\n")
                file.write("\n")
        else:
            # Root level: do NOT print bullet list or heading.
            subcommands = structure.get('subcommands', {})

        # Flags (only if we have a command chain)
        if command_chain and structure.get('flags'):
            file.write("**Flags:**\n\n")
            flag_lines = []
            for flag_dict in structure['flags']:
                flag_names = flag_dict['flag']
                description = flag_dict['description']

                # Attempt to parse a type from the first word if present
                desc_match = re.match(r'^(\w+)\s+(.*)', description)
                if desc_match:
                    flag_type = desc_match.group(1)
                    flag_desc = desc_match.group(2)
                else:
                    flag_type = ''
                    flag_desc = description

                if flag_type:
                    flag_line = f"{flag_names} {flag_type}"
                else:
                    flag_line = flag_names

                flag_lines.append((flag_line, flag_desc))

            max_len = max(len(f[0]) for f in flag_lines) if flag_lines else 0
            file.write("```bash\n")
            for fl, fd in flag_lines:
                file.write(f"{fl.ljust(max_len)}    {fd}\n")
            file.write("```\n\n")

        # Recurse into subcommands (so their headings will appear)
        subcommands = structure.get('subcommands', {})
        for subcmd in sorted(subcommands.keys()):
            write_section(subcommands[subcmd], file, command_chain + [subcmd])

    with open(file_path, "w", encoding="utf-8") as f:
        write_section(cli_structure, f)

def main():
    cli_tool = "avalanche"  # Adjust if needed
    max_depth = 10

    # Build the nested command structure
    cli_structure = get_command_structure(cli_tool, max_depth=max_depth)
    if cli_structure:
        # Save JSON
        with open("cli_structure.json", "w", encoding="utf-8") as json_file:
            json.dump(cli_structure, json_file, indent=4)
        print("CLI structure saved to cli_structure.json")

        # Generate Markdown
        generate_markdown(cli_structure, cli_tool, "cli_structure.md")
        print("Markdown documentation saved to cli_structure.md")
    else:
        print("[ERROR] Failed to retrieve CLI structure")

if __name__ == "__main__":
    main()

