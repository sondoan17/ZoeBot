import requests
import os
import logging

# Basic logging setup
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class RiotAPI:
    def __init__(self, api_key):
        self.api_key = api_key
        self.base_url_account = "https://asia.api.riotgames.com" # For PUUID (VN is in Asia/SEA)
        self.base_url_match = "https://sea.api.riotgames.com"    # For Matches (VN/SEA servers)
        self.headers = {
            "X-Riot-Token": self.api_key
        }

    def get_puuid_by_riot_id(self, game_name, tag_line):
        """
        Get PUUID from Riot ID (Name#Tag)
        """
        url = f"{self.base_url_account}/riot/account/v1/accounts/by-riot-id/{game_name}/{tag_line}"
        try:
            response = requests.get(url, headers=self.headers)
            response.raise_for_status()
            return response.json().get("puuid")
        except requests.exceptions.RequestException as e:
            logger.error(f"Error fetching PUUID for {game_name}#{tag_line}: {e}")
            return None

    def get_match_ids_by_puuid(self, puuid, count=5):
        """
        Get list of recent match IDs
        """
        url = f"{self.base_url_match}/lol/match/v5/matches/by-puuid/{puuid}/ids"
        params = {"start": 0, "count": count}
        try:
            response = requests.get(url, headers=self.headers, params=params)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            logger.error(f"Error fetching match IDs for {puuid}: {e}")
            return []

    def get_match_details(self, match_id):
        """
        Get full details of a match
        """
        url = f"{self.base_url_match}/lol/match/v5/matches/{match_id}"
        try:
            response = requests.get(url, headers=self.headers)
            response.raise_for_status()
            return response.json()
        except requests.exceptions.RequestException as e:
            logger.error(f"Error fetching details for match {match_id}: {e}")
            return None

    def parse_match_data(self, full_data, target_puuid):
        """
        Filter and extract comprehensive data for AI analysis.
        Returns only the 5 teammates (including target) pre-filtered.
        """
        if not full_data or 'info' not in full_data:
            return None
        
        info = full_data['info']
        game_duration = info.get('gameDuration', 0)
        game_duration_minutes = game_duration / 60 if game_duration > 0 else 1
        game_mode = info.get('gameMode', 'UNKNOWN')
        
        # Find target player's team
        target_player = None
        target_team_id = None
        participants_data = []
        
        participants = info.get('participants', [])
        
        for p in participants:
            if p.get('puuid') == target_puuid:
                target_team_id = p.get('teamId')
                break
        
        if target_team_id is None:
            logger.warning("Target player not found in match participants")
            return None
        
        # Extract comprehensive data for teammates only
        for p in participants:
            if p.get('teamId') != target_team_id:
                continue
            
            challenges = p.get('challenges', {})
            
            # Build comprehensive player data
            p_data = {
                # Identity
                'championName': p.get('championName'),
                'riotIdGameName': p.get('riotIdGameName'),
                'teamPosition': p.get('teamPosition'),
                'individualPosition': p.get('individualPosition'),
                'win': p.get('win'),
                
                # 1. Combat Performance
                'kills': p.get('kills'),
                'deaths': p.get('deaths'),
                'assists': p.get('assists'),
                'kda': round(challenges.get('kda', 0), 2),
                'killParticipation': round(challenges.get('killParticipation', 0) * 100, 1),  # Convert to %
                'takedowns': challenges.get('takedowns', 0),
                'largestKillingSpree': p.get('largestKillingSpree', 0),
                'soloKills': challenges.get('soloKills', 0),
                'timeSpentDead': p.get('totalTimeSpentDead', 0),
                
                # 2. Damage Profile
                'totalDamageDealtToChampions': p.get('totalDamageDealtToChampions', 0),
                'damagePerMinute': round(challenges.get('damagePerMinute', 0), 0),
                'teamDamagePercentage': round(challenges.get('teamDamagePercentage', 0) * 100, 1),  # %
                'physicalDamageDealtToChampions': p.get('physicalDamageDealtToChampions', 0),
                'magicDamageDealtToChampions': p.get('magicDamageDealtToChampions', 0),
                'trueDamageDealtToChampions': p.get('trueDamageDealtToChampions', 0),
                'timeCCingOthers': p.get('timeCCingOthers', 0),
                'totalTimeCCDealt': p.get('totalTimeCCDealt', 0),
                
                # 3. Laning & Economy
                'laneMinionsFirst10Minutes': challenges.get('laneMinionsFirst10Minutes', 0),
                'totalCS': p.get('totalMinionsKilled', 0) + p.get('neutralMinionsKilled', 0),
                'csPerMinute': round((p.get('totalMinionsKilled', 0) + p.get('neutralMinionsKilled', 0)) / game_duration_minutes, 1),
                'goldEarned': p.get('goldEarned', 0),
                'goldPerMinute': round(challenges.get('goldPerMinute', 0), 0),
                'champLevel': p.get('champLevel', 0),
                'maxCsAdvantageOnLaneOpponent': round(challenges.get('maxCsAdvantageOnLaneOpponent', 0), 0),
                'maxLevelLeadLaneOpponent': challenges.get('maxLevelLeadLaneOpponent', 0),
                
                # 4. Macro & Objectives
                'dragonKills': p.get('dragonKills', 0),
                'dragonTakedowns': challenges.get('dragonTakedowns', 0),
                'baronKills': p.get('baronKills', 0),
                'baronTakedowns': challenges.get('baronTakedowns', 0),
                'damageDealtToObjectives': p.get('damageDealtToObjectives', 0),
                'turretTakedowns': challenges.get('turretTakedowns', 0),
                'inhibitorTakedowns': p.get('inhibitorTakedowns', 0),
                'teamBaronKills': challenges.get('teamBaronKills', 0),
                'teamElderDragonKills': challenges.get('teamElderDragonKills', 0),
                
                # 5. Vision Control
                'visionScore': p.get('visionScore', 0),
                'visionScorePerMinute': round(challenges.get('visionScorePerMinute', 0), 2),
                'wardsPlaced': p.get('wardsPlaced', 0),
                'controlWardsPlaced': challenges.get('controlWardsPlaced', 0),
                'wardsKilled': p.get('wardsKilled', 0),
                'wardTakedowns': challenges.get('wardTakedowns', 0),
                
                # 6. Mechanics
                'skillshotsHit': challenges.get('skillshotsHit', 0),
                'skillshotsDodged': challenges.get('skillshotsDodged', 0),
                'abilityUses': challenges.get('abilityUses', 0),
                'dodgeSkillShotsSmallWindow': challenges.get('dodgeSkillShotsSmallWindow', 0),
            }

            participants_data.append(p_data)
            
            if p.get('puuid') == target_puuid:
                target_player = p_data

        return {
            'matchId': full_data.get('metadata', {}).get('matchId'),
            'gameDuration': game_duration,
            'gameDurationMinutes': round(game_duration_minutes, 1),
            'gameMode': game_mode,
            'win': target_player.get('win') if target_player else None,
            'target_player_name': target_player.get('riotIdGameName') if target_player else None,
            'teammates': participants_data
        }
