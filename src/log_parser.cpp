/*
 * Log Parser - C++ helper for k8s-pod-analyzer
 * Parses and filters Kubernetes pod logs
 */

#include <iostream>
#include <string>
#include <vector>
#include <fstream>
#include <sstream>
#include <algorithm>
#include <regex>

struct LogEntry {
    std::string timestamp;
    std::string level;
    std::string message;
    std::string pod;
    std::string container;
};

class LogParser {
private:
    std::vector<LogEntry> entries;
    
public:
    void parseLine(const std::string& line) {
        LogEntry entry;
        
        // Simple parsing - extract timestamp, level, message
        std::regex pattern(R"(\[(.*?)\]\s+(\w+):\s+(.*))");
        std::smatch matches;
        
        if (std::regex_match(line, matches, pattern)) {
            entry.timestamp = matches[1];
            entry.level = matches[2];
            entry.message = matches[3];
            entries.push_back(entry);
        }
    }
    
    void parseFile(const std::string& filename) {
        std::ifstream file(filename);
        std::string line;
        
        while (std::getline(file, line)) {
            parseLine(line);
        }
    }
    
    std::vector<LogEntry> filterByLevel(const std::string& level) const {
        std::vector<LogEntry> filtered;
        std::copy_if(entries.begin(), entries.end(), 
                     std::back_inserter(filtered),
                     [&level](const LogEntry& e) { 
                         return e.level == level; 
                     });
        return filtered;
    }
    
    void printStats() const {
        std::cout << "Log Statistics:" << std::endl;
        std::cout << "Total entries: " << entries.size() << std::endl;
        
        // Count by level
        std::map<std::string, int> levelCount;
        for (const auto& entry : entries) {
            levelCount[entry.level]++;
        }
        
        for (const auto& pair : levelCount) {
            std::cout << pair.first << ": " << pair.second << std::endl;
        }
    }
    
    void printEntries(const std::vector<LogEntry>& entries) const {
        for (const auto& entry : entries) {
            std::cout << "[" << entry.timestamp << "] " 
                      << entry.level << ": " << entry.message << std::endl;
        }
    }
};

int main(int argc, char* argv[]) {
    if (argc < 2) {
        std::cerr << "Usage: " << argv[0] << " <logfile>" << std::endl;
        return 1;
    }
    
    LogParser parser;
    parser.parseFile(argv[1]);
    parser.printStats();
    
    return 0;
}
