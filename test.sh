curl http://localhost:11434/api/generate -H "Content-Type: application/json" -d '{
  "model": "phi",
  "prompt": "Extract job_type, skills, and description (as a single string, minimum 250 characters) from this job posting. Return valid JSON only. Do not split description across keys:\n\n
About the job

What To Expect
Tesla is seeking a passionate Mechanical Design Intern to lead the successful bring-up and sustaining of automated test equipment in our state-of-the-art Gigafactory Berlin facility. The candidate will help design, validate, and deploy next generation manufacturing test technology, that ensures high quality production of supercharging, energy, and battery systems.

Location: Gigafactory Berlin Brandenburg

Duration: 6 months

Time type: Full Time

Start date: March 2025

What You'll Do


Design mechanical systems for manufacturing end of line test applications of various Tesla products including Superchargers, Power Electronics, Battery Packs
Develop new concepts for complex connector designs that interface electrically & mechanically with device under test
Continuous improvement of innovative technologies for efficient testing and unit defect detection capabilities
Provide Design for manufacture feedback to product design teams to ensure compatibility and ease of deployment of our test systems
Drive fabrication and validation of designed systems with support from technician team
Work with cross-functional teams product firmware, process eng., quality eng., manufacturing eng., product design.. to sustain test equipment as product/process/equipment changes are needed
Troubleshoot and sustain test equipment in production environment while driving long term improvements on equipment reliability
Train production, sustaining engineering and technician teams to maintain equipment post deployment



What You'll Bring


Pursuing/recently pursued a degree in mechanical engineering or similar
Solidworks CAD design and drafting skills are a must.
Experience in machine design in manufacturing industry
Familiar with mechanical loading analysis methodologies
Exposure to thermal modeling and analysis
Experienced with GD&T best practices
Strong mechanical and electrical hardware troubleshooting skills
Remain engaged, proactive, and positive in tough circumstances, owning assignments and taking full accountability for overall team success.",
  "stream": false
}'
