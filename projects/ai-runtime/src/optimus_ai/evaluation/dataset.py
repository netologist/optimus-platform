from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass
class GoldenScenario:
    """A hand-crafted golden evaluation scenario representing an enterprise asset failure."""

    scenario_id: str
    name: str
    tenant_id: str
    asset_id: str
    symptom: str
    expected_tools: list[str] = field(
        default_factory=lambda: [
            "eam.get_maintenance_history",
            "plm.search_documents",
            "erp.get_inventory",
        ]
    )
    expected_min_failures: int = 1
    expected_spare_part_id: str = "SP-COOL-9981"
    expected_spare_in_stock: bool = True
    expected_severity: str = "P1"
    expected_safety_risk: str = "HIGH"
    expected_field_visit: bool = True
    mock_history: dict[str, Any] = field(default_factory=dict)
    mock_plm: dict[str, Any] = field(default_factory=dict)
    mock_inventory: dict[str, Any] = field(default_factory=dict)


GOLDEN_DATASET: list[GoldenScenario] = [
    GoldenScenario(
        scenario_id="P104-OVERHEAT",
        name="Pump P-104 Recurrent Overheating (Manchester Plant)",
        tenant_id="acme",
        asset_id="P-104",
        symptom="repeated overheating under peak pumping load",
        expected_tools=[
            "eam.get_maintenance_history",
            "plm.search_documents",
            "erp.get_inventory",
        ],
        expected_min_failures=4,
        expected_spare_part_id="SP-COOL-9981",
        expected_spare_in_stock=True,
        expected_severity="P1",
        expected_safety_risk="HIGH",
        expected_field_visit=True,
        mock_history={"failures_last_30_days": 4, "last_maintenance": "2026-09-22"},
        mock_plm={
            "document_id": "PLM-COOL-4021",
            "section": "§4.2",
            "content": "Thermostat bypass valve sticking causes coolant circulation failure.",
            "spare_part": "SP-COOL-9981",
        },
        mock_inventory={"spare_part_id": "SP-COOL-9981", "in_stock": 6, "warehouse": "Manchester"},
    ),
    GoldenScenario(
        scenario_id="T200-VIBRATION",
        name="Gas Turbine T-200 Bearing High Vibration (Leeds Plant)",
        tenant_id="acme",
        asset_id="T-200",
        symptom="excessive radial bearing vibration exceeding ISO limit",
        expected_tools=[
            "eam.get_maintenance_history",
            "plm.search_documents",
            "erp.get_inventory",
        ],
        expected_min_failures=3,
        expected_spare_part_id="SP-TURB-5520",
        expected_spare_in_stock=True,
        expected_severity="P1",
        expected_safety_risk="HIGH",
        expected_field_visit=True,
        mock_history={"failures_last_30_days": 3, "last_maintenance": "2026-09-18"},
        mock_plm={
            "document_id": "PLM-TURB-9011",
            "section": "§8.1",
            "content": "Journal bearing sleeve micro-cracking and hydrodynamic oil wedge collapse.",
            "spare_part": "SP-TURB-5520",
        },
        mock_inventory={"spare_part_id": "SP-TURB-5520", "in_stock": 2, "warehouse": "Leeds"},
    ),
    GoldenScenario(
        scenario_id="V301-SEAL-LEAK",
        name="Control Valve V-301 Hydraulic Seal Leakage (Birmingham)",
        tenant_id="acme",
        asset_id="V-301",
        symptom="hydraulic pressure drop and fluid seepage at actuator stem",
        expected_tools=[
            "eam.get_maintenance_history",
            "plm.search_documents",
            "erp.get_inventory",
        ],
        expected_min_failures=2,
        expected_spare_part_id="SP-VALV-1200",
        expected_spare_in_stock=True,
        expected_severity="P2",
        expected_safety_risk="MEDIUM",
        expected_field_visit=True,
        mock_history={"failures_last_30_days": 2, "last_maintenance": "2026-09-10"},
        mock_plm={
            "document_id": "PLM-VALV-3310",
            "section": "§3.4",
            "content": "Stem packing extrusion resulting from thermal cycling above 180°C.",
            "spare_part": "SP-VALV-1200",
        },
        mock_inventory={"spare_part_id": "SP-VALV-1200", "in_stock": 14, "warehouse": "Birmingham"},
    ),
    GoldenScenario(
        scenario_id="C101-MOTOR-STALL",
        name="Conveyor C-101 Induction Motor Stall (Sheffield)",
        tenant_id="acme",
        asset_id="C-101",
        symptom="sudden stator overcurrent trip during startup",
        expected_tools=[
            "eam.get_maintenance_history",
            "plm.search_documents",
            "erp.get_inventory",
        ],
        expected_min_failures=2,
        expected_spare_part_id="SP-MOT-8812",
        expected_spare_in_stock=False,
        expected_severity="P2",
        expected_safety_risk="LOW",
        expected_field_visit=True,
        mock_history={"failures_last_30_days": 2, "last_maintenance": "2026-09-15"},
        mock_plm={
            "document_id": "PLM-MOT-2044",
            "section": "§2.1",
            "content": "Stator insulation degradation and inter-turn short circuiting.",
            "spare_part": "SP-MOT-8812",
        },
        mock_inventory={"spare_part_id": "SP-MOT-8812", "in_stock": 0, "warehouse": "Sheffield"},
    ),
    GoldenScenario(
        scenario_id="K500-COMPRESSOR-PRESSURE",
        name="Screw Compressor K-500 Differential Pressure Loss (Newcastle)",
        tenant_id="acme",
        asset_id="K-500",
        symptom="discharge pressure lower than design threshold under full load",
        expected_tools=[
            "eam.get_maintenance_history",
            "plm.search_documents",
            "erp.get_inventory",
        ],
        expected_min_failures=3,
        expected_spare_part_id="SP-COMP-7740",
        expected_spare_in_stock=True,
        expected_severity="P1",
        expected_safety_risk="HIGH",
        expected_field_visit=True,
        mock_history={"failures_last_30_days": 3, "last_maintenance": "2026-09-05"},
        mock_plm={
            "document_id": "PLM-COMP-6112",
            "section": "§5.3",
            "content": "Rotor profile wear and internal recirculation across suction bypass.",
            "spare_part": "SP-COMP-7740",
        },
        mock_inventory={"spare_part_id": "SP-COMP-7740", "in_stock": 3, "warehouse": "Newcastle"},
    ),
    GoldenScenario(
        scenario_id="R402-REACTOR-AGITATOR",
        name="Chemical Reactor R-402 Agitator Seal Failure (Bristol)",
        tenant_id="acme",
        asset_id="R-402",
        symptom="mechanical seal barrier fluid leakage and abnormal torque reading",
        expected_tools=[
            "eam.get_maintenance_history",
            "plm.search_documents",
            "erp.get_inventory",
        ],
        expected_min_failures=2,
        expected_spare_part_id="SP-SEAL-9010",
        expected_spare_in_stock=True,
        expected_severity="P1",
        expected_safety_risk="HIGH",
        expected_field_visit=True,
        mock_history={"failures_last_30_days": 2, "last_maintenance": "2026-09-12"},
        mock_plm={
            "document_id": "PLM-SEAL-7701",
            "section": "§6.2",
            "content": "Tandem cartridge mechanical seal face blistering due to dry running.",
            "spare_part": "SP-SEAL-9010",
        },
        mock_inventory={"spare_part_id": "SP-SEAL-9010", "in_stock": 1, "warehouse": "Bristol"},
    ),
    GoldenScenario(
        scenario_id="F109-EXHAUST-FAN",
        name="Exhaust Fan F-109 Impeller Imbalance (Edinburgh)",
        tenant_id="acme",
        asset_id="F-109",
        symptom="harmonic casing resonance and blade particulate accumulation",
        expected_tools=[
            "eam.get_maintenance_history",
            "plm.search_documents",
            "erp.get_inventory",
        ],
        expected_min_failures=1,
        expected_spare_part_id="SP-FAN-3100",
        expected_spare_in_stock=True,
        expected_severity="P3",
        expected_safety_risk="LOW",
        expected_field_visit=False,
        mock_history={"failures_last_30_days": 1, "last_maintenance": "2026-08-30"},
        mock_plm={
            "document_id": "PLM-FAN-1100",
            "section": "§1.8",
            "content": "Dynamic balance correction via counterweight addition on outer rim.",
            "spare_part": "SP-FAN-3100",
        },
        mock_inventory={"spare_part_id": "SP-FAN-3100", "in_stock": 8, "warehouse": "Edinburgh"},
    ),
    GoldenScenario(
        scenario_id="B701-BOILER-PUMP",
        name="Steam Boiler Feedwater Pump B-701 Cavitation (Glasgow)",
        tenant_id="acme",
        asset_id="B-701",
        symptom="acoustic popping noise and suction pressure fluctuation",
        expected_tools=[
            "eam.get_maintenance_history",
            "plm.search_documents",
            "erp.get_inventory",
        ],
        expected_min_failures=4,
        expected_spare_part_id="SP-PUMP-4420",
        expected_spare_in_stock=True,
        expected_severity="P1",
        expected_safety_risk="HIGH",
        expected_field_visit=True,
        mock_history={"failures_last_30_days": 4, "last_maintenance": "2026-09-20"},
        mock_plm={
            "document_id": "PLM-BOIL-8221",
            "section": "§4.5",
            "content": "NPSH margin deficit causing impeller eye erosion and pitting.",
            "spare_part": "SP-PUMP-4420",
        },
        mock_inventory={"spare_part_id": "SP-PUMP-4420", "in_stock": 4, "warehouse": "Glasgow"},
    ),
    GoldenScenario(
        scenario_id="G602-GENERATOR-EXCITATION",
        name="Emergency Generator G-602 Excitation Tripping (Cardiff)",
        tenant_id="acme",
        asset_id="G-602",
        symptom="under-voltage cutoff during automatic transfer switch engagement",
        expected_tools=[
            "eam.get_maintenance_history",
            "plm.search_documents",
            "erp.get_inventory",
        ],
        expected_min_failures=2,
        expected_spare_part_id="SP-GEN-1055",
        expected_spare_in_stock=True,
        expected_severity="P1",
        expected_safety_risk="HIGH",
        expected_field_visit=True,
        mock_history={"failures_last_30_days": 2, "last_maintenance": "2026-09-08"},
        mock_plm={
            "document_id": "PLM-GEN-5002",
            "section": "§3.1",
            "content": "Rotating rectifier diode failure causing loss of field excitation.",
            "spare_part": "SP-GEN-1055",
        },
        mock_inventory={"spare_part_id": "SP-GEN-1055", "in_stock": 5, "warehouse": "Cardiff"},
    ),
    GoldenScenario(
        scenario_id="X805-HEAT-EXCHANGER",
        name="Shell & Tube Heat Exchanger X-805 Thermal Fouling (Belfast)",
        tenant_id="acme",
        asset_id="X-805",
        symptom="temperature cross violation and cooling water side pressure drop",
        expected_tools=[
            "eam.get_maintenance_history",
            "plm.search_documents",
            "erp.get_inventory",
        ],
        expected_min_failures=1,
        expected_spare_part_id="SP-HEX-6600",
        expected_spare_in_stock=True,
        expected_severity="P3",
        expected_safety_risk="LOW",
        expected_field_visit=False,
        mock_history={"failures_last_30_days": 1, "last_maintenance": "2026-08-15"},
        mock_plm={
            "document_id": "PLM-HEX-4100",
            "section": "§2.7",
            "content": "Tube bundle bio-fouling and mineral scaling requiring acid clean.",
            "spare_part": "SP-HEX-6600",
        },
        mock_inventory={"spare_part_id": "SP-HEX-6600", "in_stock": 20, "warehouse": "Belfast"},
    ),
]
