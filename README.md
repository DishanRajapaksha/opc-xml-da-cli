# OPC XML DA CLI Tool

A command-line interface tool for interacting with OPC XML Data Access (DA) servers. This tool provides a convenient way to connect to OPC XML DA servers, browse server information, read and write data, and manage subscriptions.

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Usage](#usage)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

## Overview

OPC XML DA CLI is a lightweight command-line tool designed to simplify interaction with OPC XML Data Access servers. Whether you're testing server connectivity, debugging data flow, or automating data collection, this tool provides the functionality you need.

### Key Features

- **Server Connection Management**: Connect to and authenticate with OPC XML DA servers
- **Server Browsing**: Explore server structure and available items
- **Data Reading**: Read current values from OPC items
- **Data Writing**: Write values to OPC items
- **Subscription Management**: Create and manage real-time data subscriptions
- **Batch Operations**: Process multiple items in a single operation
- **Output Formatting**: Multiple output formats (JSON, CSV, table)

## Installation

### Prerequisites

- .NET 6.0 or later
- Windows, macOS, or Linux
- Network access to OPC XML DA server

### From Source

1. Clone the repository:
```bash
git clone https://github.com/DishanRajapaksha/opc-xml-da-cli.git
cd opc-xml-da-cli
```

2. Build the project:
```bash
dotnet build -c Release
```

3. Install globally:
```bash
dotnet pack -c Release
dotnet tool install --global --add-source ./nupkg opc-xml-da-cli
```

### From NuGet (if published)

```bash
dotnet tool install --global opc-xml-da-cli
```

### Verify Installation

```bash
opc-da-cli --version
opc-da-cli --help
```

## Quick Start

### Basic Connection

Connect to a local OPC XML DA server:

```bash
opc-da-cli connect http://localhost:8080/OpcXmlDa.asmx
```

### List Available Items

Browse server for available items:

```bash
opc-da-cli list-items
```

### Read a Value

Read the current value of an item:

```bash
opc-da-cli read --item "MyItem"
```

### Write a Value

Write a value to an item:

```bash
opc-da-cli write --item "MyItem" --value "42"
```

## Usage

### Command Structure

```
opc-da-cli [command] [options]
```

### Global Options

- `--server <url>`: OPC XML DA server URL (or set via environment variable `OPC_SERVER_URL`)
- `--username <user>`: Username for authentication
- `--password <pass>`: Password for authentication
- `--timeout <seconds>`: Request timeout in seconds (default: 30)
- `--verbose`: Enable verbose logging
- `--format <format>`: Output format (json, csv, table) (default: table)
- `--help`: Show help information
- `--version`: Show version information

### Commands

#### connect
Establish and validate connection to an OPC server.

```bash
opc-da-cli connect <server-url> [options]
```

Options:
- `--username <user>`: Username for authentication
- `--password <pass>`: Password for authentication
- `--test`: Test the connection and display server info

Example:
```bash
opc-da-cli connect http://opcserver.example.com:8080/OpcXmlDa.asmx --test
```

#### list-items
Browse and list available items on the server.

```bash
opc-da-cli list-items [options]
```

Options:
- `--path <path>`: Browse path (default: root)
- `--max-items <number>`: Maximum items to return (default: 100)
- `--recursive`: Recursively list all items

Example:
```bash
opc-da-cli list-items --path "Temperature" --recursive
```

#### read
Read values from one or more items.

```bash
opc-da-cli read [options]
```

Options:
- `--item <name>`: Single item name
- `--items <file>`: File containing list of items (one per line)
- `--include-timestamp`: Include timestamp in output
- `--include-quality`: Include data quality status

Example:
```bash
opc-da-cli read --item "Sensor.Temperature" --include-timestamp --include-quality
```

#### write
Write values to OPC items.

```bash
opc-da-cli write [options]
```

Options:
- `--item <name>`: Item name (required)
- `--value <value>`: Value to write (required)
- `--data-type <type>`: Data type (int, float, string, bool)
- `--confirm`: Prompt for confirmation before writing

Example:
```bash
opc-da-cli write --item "MotorSpeed" --value "1500" --data-type int --confirm
```

#### subscribe
Create a subscription for real-time updates.

```bash
opc-da-cli subscribe [options]
```

Options:
- `--items <file>`: File containing list of items
- `--interval <ms>`: Update interval in milliseconds (default: 1000)
- `--duration <seconds>`: Subscription duration (0 for infinite)
- `--output-file <path>`: Write updates to file instead of console

Example:
```bash
opc-da-cli subscribe --items items.txt --interval 500 --duration 300
```

#### server-info
Display server information and capabilities.

```bash
opc-da-cli server-info [options]
```

Example:
```bash
opc-da-cli server-info --verbose
```

#### export
Export items and values to a file.

```bash
opc-da-cli export [options]
```

Options:
- `--output <file>`: Output file path (required)
- `--format <format>`: Format (json, csv, xml) (default: json)
- `--items <file>`: File containing list of items to export
- `--include-metadata`: Include item metadata

Example:
```bash
opc-da-cli export --output data.json --format json --include-metadata
```

## Examples

### Example 1: Connect and Read Temperature

```bash
# Connect to server
opc-da-cli connect http://plc.example.com:8080/OpcXmlDa.asmx --test

# Read temperature value
opc-da-cli read --item "Building.Floor1.Room101.Temperature" --include-timestamp
```

Output:
```
Item: Building.Floor1.Room101.Temperature
Value: 22.5
Quality: Good
Timestamp: 2025-12-19T10:30:45Z
```

### Example 2: Batch Read Multiple Items

Create `items.txt`:
```
Building.Floor1.Temperature
Building.Floor1.Humidity
Building.Floor1.CO2Level
Building.Floor2.Temperature
Building.Floor2.Humidity
```

Execute:
```bash
opc-da-cli read --items items.txt --include-quality --format csv
```

Output:
```
Item,Value,Quality,Timestamp
Building.Floor1.Temperature,22.5,Good,2025-12-19T10:30:45Z
Building.Floor1.Humidity,45.3,Good,2025-12-19T10:30:45Z
Building.Floor1.CO2Level,420,Good,2025-12-19T10:30:45Z
Building.Floor2.Temperature,21.8,Good,2025-12-19T10:30:45Z
Building.Floor2.Humidity,48.2,Good,2025-12-19T10:30:45Z
```

### Example 3: Monitor Real-Time Sensor Data

```bash
# Create subscription file
cat > sensors.txt << EOF
Sensors.Pressure
Sensors.Temperature
Sensors.Flow
EOF

# Subscribe with 500ms updates for 5 minutes
opc-da-cli subscribe --items sensors.txt --interval 500 --duration 300
```

### Example 4: Write with Confirmation

```bash
opc-da-cli write \
  --item "Equipment.Pump1.Speed" \
  --value "1200" \
  --data-type int \
  --confirm
```

Prompt:
```
About to write value to Equipment.Pump1.Speed
New value: 1200
Continue? (y/n): y
Write successful
Item: Equipment.Pump1.Speed
New Value: 1200
```

### Example 5: Export Data with Metadata

```bash
opc-da-cli export \
  --output database_export.json \
  --format json \
  --include-metadata \
  --format table

# Then use the exported file for backup or analysis
```

### Example 6: Environment Variables

```bash
# Set server URL as environment variable
export OPC_SERVER_URL="http://opcserver.example.com:8080/OpcXmlDa.asmx"
export OPC_USERNAME="admin"
export OPC_PASSWORD="secure_password"

# No need to specify server in each command
opc-da-cli read --item "Item1"
opc-da-cli list-items --recursive
```

### Example 7: Troubleshooting Connection

```bash
# Test connection with verbose output
opc-da-cli connect http://opcserver.example.com:8080/OpcXmlDa.asmx \
  --test \
  --verbose

# Output includes:
# - Server version
# - Available services
# - Supported data types
# - Authentication methods
```

## Troubleshooting

### Connection Issues

#### Problem: "Unable to connect to server"

**Solutions:**
1. Verify the server URL is correct and accessible:
   ```bash
   ping opcserver.example.com
   curl http://opcserver.example.com:8080/OpcXmlDa.asmx
   ```

2. Check firewall rules:
   ```bash
   # Windows
   netstat -an | find "8080"
   
   # Linux/macOS
   netstat -an | grep 8080
   ```

3. Test with credentials:
   ```bash
   opc-da-cli connect http://opcserver.example.com:8080/OpcXmlDa.asmx \
     --username admin \
     --password password \
     --test
   ```

#### Problem: "Request timeout"

**Solutions:**
1. Increase timeout:
   ```bash
   opc-da-cli read --item "Item1" --timeout 60
   ```

2. Check server performance and network latency
3. Reduce the number of items being read at once

### Authentication Issues

#### Problem: "Authentication failed"

**Solutions:**
1. Verify credentials:
   ```bash
   opc-da-cli connect <url> --username <user> --password <pass> --test
   ```

2. Check username/password for special characters that need escaping:
   ```bash
   # Use quotes for passwords with spaces or special characters
   opc-da-cli connect <url> --password "p@ssw0rd!special"
   ```

3. Verify user has OPC read/write permissions on the server

### Data Access Issues

#### Problem: "Item not found"

**Solutions:**
1. List available items:
   ```bash
   opc-da-cli list-items --recursive
   ```

2. Verify item name is correct (case-sensitive):
   ```bash
   # Wrong: item
   # Correct: Item.Name.Path
   ```

3. Check item access permissions

#### Problem: "Access denied" when writing

**Solutions:**
1. Verify user has write permissions for the item
2. Check if item is read-only:
   ```bash
   opc-da-cli server-info --verbose
   ```

3. Test with a different item first

### Output and Format Issues

#### Problem: "Invalid format specified"

**Solutions:**
1. Use supported formats only: `json`, `csv`, `table`
   ```bash
   opc-da-cli read --item "Item1" --format json
   ```

2. Check file permissions when using `--output-file`

### Performance Issues

#### Problem: "Tool is slow or consuming high memory"

**Solutions:**
1. Reduce number of items read at once
2. Adjust subscription intervals:
   ```bash
   opc-da-cli subscribe --items items.txt --interval 2000
   ```

3. Use filtering or batching:
   ```bash
   # Split large item lists
   split -l 100 large_items.txt items_
   ```

### Debug Mode

Enable verbose logging for detailed troubleshooting:

```bash
opc-da-cli --verbose read --item "Item1"
```

This will output:
- HTTP request/response details
- XML parsing information
- Authentication process details
- Timing information

### Getting Help

For additional support:

1. Check the help for specific commands:
   ```bash
   opc-da-cli read --help
   opc-da-cli write --help
   ```

2. Review server logs for OPC XML DA service errors

3. Enable network packet capture for protocol-level debugging:
   ```bash
   # Linux/macOS
   tcpdump -i any -n "host opcserver.example.com and port 8080"
   ```

4. Check .NET runtime logs if available

### Common Error Codes

- `E001`: Server connection failed
- `E002`: Authentication failed
- `E003`: Item not found
- `E004`: Access denied
- `E005`: Invalid data type
- `E006`: Value out of range
- `E007`: Server error
- `E008`: Timeout

## Contributing

Contributions are welcome! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues, questions, or suggestions:
- Open an issue on GitHub
- Check existing issues for solutions
- Review the troubleshooting section above

---

**Last Updated:** 2025-12-19

For the latest version and updates, visit: https://github.com/DishanRajapaksha/opc-xml-da-cli
