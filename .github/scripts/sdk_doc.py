import os
import re
from typing import Dict, List, Optional, Set
from dataclasses import dataclass
import ast
import glob

@dataclass
class GoParam:
    name: str
    type: str
    description: str = ""

@dataclass
class GoReturn:
    type: str
    description: str = ""

class GoFunction:
    def __init__(self, name: str, signature: str, doc: str):
        self.name = name
        self.signature = signature
        self.description = ""
        self.params: List[GoParam] = []
        self.returns: List[GoReturn] = []
        self.examples: List[str] = []
        self.notes: List[str] = []
        self.parse_doc(doc)

    def parse_doc(self, doc: str):
        if not doc:
            return

        sections = doc.split("\n")
        current_section = "description"
        current_example = []
        
        for line in sections:
            line = line.strip()
            if not line:
                continue
                
            # Parse section markers
            if line.lower().startswith("@param"):
                parts = line[6:].strip().split(" ", 2)
                if len(parts) >= 2:
                    name, type_str = parts[0], parts[1]
                    desc = parts[2] if len(parts) > 2 else ""
                    self.params.append(GoParam(name, type_str, desc))
                continue
                
            if line.lower().startswith("@return"):
                parts = line[7:].strip().split(" ", 1)
                type_str = parts[0]
                desc = parts[1] if len(parts) > 1 else ""
                self.returns.append(GoReturn(type_str, desc))
                continue
                
            if line.lower().startswith("example:"):
                current_section = "example"
                continue
                
            if line.lower().startswith("note:"):
                self.notes.append(line[5:].strip())
                continue

            # Handle content based on current section
            if current_section == "example":
                if line.startswith("```") and current_example:
                    self.examples.append("\n".join(current_example))
                    current_example = []
                    current_section = "description"
                elif not line.startswith("```"):
                    current_example.append(line)
            else:
                if not self.description:
                    self.description = line
                else:
                    self.description += " " + line

class GoPackage:
    def __init__(self, name: str, path: str):
        self.name = name
        self.path = path
        self.functions: List[GoFunction] = []
        self.description = ""
        self.examples: List[str] = []

def parse_go_file(file_path: str) -> List[GoFunction]:
    """Parse a Go file and extract detailed function documentation."""
    functions = []
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()

    # Regular expression for matching Go function declarations with docs
    func_pattern = r'(?:/\*\*(.*?)\*/\s*)?func\s+(\w+)\s*\((.*?)\)(?:\s*\(?(.*?)\)?)?\s*{'
    matches = re.finditer(func_pattern, content, re.DOTALL)

    for match in matches:
        doc = match.group(1).strip() if match.group(1) else ""
        name = match.group(2)
        params = match.group(3)
        returns = match.group(4) if match.group(4) else ""

        signature = f"func {name}({params})"
        if returns:
            signature += f" ({returns})"

        functions.append(GoFunction(name, signature, doc))

    return functions

def scan_sdk_packages(sdk_path: str) -> Dict[str, GoPackage]:
    """Scan the SDK directory for Go packages."""
    packages = {}
    
    for root, _, files in os.walk(sdk_path):
        go_files = [f for f in files if f.endswith('.go')]
        if not go_files:
            continue

        # Get package name from relative path
        rel_path = os.path.relpath(root, sdk_path)
        package_name = os.path.basename(rel_path)
        
        if package_name not in packages:
            packages[package_name] = GoPackage(package_name, rel_path)

        # Parse package description from package comment in any go file
        for go_file in go_files:
            file_path = os.path.join(root, go_file)
            with open(file_path, 'r', encoding='utf-8') as f:
                content = f.read()
                pkg_doc_match = re.search(r'/\*\*(.*?)\*/\s*package', content, re.DOTALL)
                if pkg_doc_match:
                    packages[package_name].description = pkg_doc_match.group(1).strip()
            
            # Parse functions from the file
            functions = parse_go_file(file_path)
            packages[package_name].functions.extend(functions)

    return packages

def generate_single_markdown(packages: Dict[str, GoPackage], output_file: str):
    """Generate a single markdown documentation file for all packages."""
    with open(output_file, 'w', encoding='utf-8') as f:
        f.write('# SDK Documentation\n\n')
        
        # Generate table of contents
        f.write('## Table of Contents\n\n')
        for package_name in sorted(packages.keys()):
            f.write(f'- [{package_name}](#{package_name.lower()})\n')
        f.write('\n---\n\n')

        # Generate package documentation
        for package_name, package in sorted(packages.items()):
            f.write(f'## {package_name}\n\n')
            
            if package.description:
                f.write(f'{package.description}\n\n')

            if package.examples:
                f.write('### Package Examples\n\n')
                for example in package.examples:
                    f.write('```go\n')
                    f.write(example)
                    f.write('\n```\n\n')

            f.write('### Functions\n\n')
            for func in sorted(package.functions, key=lambda x: x.name):
                f.write(f'#### {func.name}\n\n')
                f.write('```go\n')
                f.write(f'{func.signature}\n')
                f.write('```\n\n')
                
                if func.description:
                    f.write(f'{func.description}\n\n')

                if func.params:
                    f.write('**Parameters:**\n\n')
                    for param in func.params:
                        f.write(f'- `{param.name}` ({param.type})')
                        if param.description:
                            f.write(f': {param.description}')
                        f.write('\n')
                    f.write('\n')

                if func.returns:
                    f.write('**Returns:**\n\n')
                    for ret in func.returns:
                        f.write(f'- ({ret.type})')
                        if ret.description:
                            f.write(f': {ret.description}')
                        f.write('\n')
                    f.write('\n')

                if func.examples:
                    f.write('**Examples:**\n\n')
                    for example in func.examples:
                        f.write('```go\n')
                        f.write(example)
                        f.write('\n```\n\n')

                if func.notes:
                    f.write('**Notes:**\n\n')
                    for note in func.notes:
                        f.write(f'- {note}\n')
                    f.write('\n')
            
            f.write('---\n\n')

def main():
    # Get the absolute path to the SDK directory
    script_dir = os.path.dirname(os.path.abspath(__file__))
    sdk_path = os.path.abspath(os.path.join(script_dir, '..', '..', 'sdk'))
    output_file = os.path.join(sdk_path, 'SDK.md')

    print(f"Scanning SDK directory: {sdk_path}")
    packages = scan_sdk_packages(sdk_path)
    
    print(f"Generating documentation in: {output_file}")
    generate_single_markdown(packages, output_file)
    
    print("Documentation generation complete!")

if __name__ == "__main__":
    main()
